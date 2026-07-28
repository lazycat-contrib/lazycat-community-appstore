package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v89/github"
	"golang.org/x/mod/semver"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/githublpkupdatepolicy"
	"lazycat.community/appstore/ent/reviewrequest"
)

const (
	defaultGitHubLPKUpdateIntervalMinutes = 24 * 60
	minGitHubLPKUpdateIntervalMinutes     = 60
	maxGitHubLPKUpdateIntervalMinutes     = 30 * 24 * 60
	githubLPKUpdateScanInterval           = time.Minute
	githubLPKUpdateBatchSize              = 8
	githubLPKUpdateClaimReleaseTimeout    = 2 * time.Second
)

var releaseVersionPattern = regexp.MustCompile(`(?i)(\d+\.\d+\.\d+[0-9a-z.+-]*)$`)

var errGitHubLPKUpdateUnsupported = errors.New("the latest published version is not a supported GitHub Release LPK URL")
var errGitHubLPKUpdateDisabled = errors.New("automatic GitHub LPK updates are disabled")

type githubLatestReleaseClient interface {
	GetLatestRelease(context.Context, string, string) (*github.RepositoryRelease, *github.Response, error)
}

type githubReleaseLPK struct {
	Owner     string
	Repo      string
	Tag       string
	AssetName string
	URL       string
}

type githubLPKUpdateScheduler struct {
	server *Server
	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
	once   sync.Once
}

type githubLPKUpdatePolicyDTO struct {
	Enabled         bool       `json:"enabled"`
	IntervalMinutes int        `json:"intervalMinutes"`
	LastCheckedAt   *time.Time `json:"lastCheckedAt,omitempty"`
	LastSuccessAt   *time.Time `json:"lastSuccessAt,omitempty"`
	NextCheckAt     *time.Time `json:"nextCheckAt,omitempty"`
	LastVersion     string     `json:"lastVersion,omitempty"`
	LastError       string     `json:"lastError,omitempty"`
}

type updateGitHubLPKUpdatePolicyRequest struct {
	Enabled         bool `json:"enabled"`
	IntervalMinutes *int `json:"intervalMinutes"`
}

type automaticGitHubVersionInput struct {
	Version             string
	Changelog           string
	DownloadURL         string
	SHA256              string
	FileSize            int64
	UpstreamPublishedAt *time.Time
}

func newGitHubLPKUpdateScheduler(server *Server) (*githubLPKUpdateScheduler, error) {
	if server == nil || server.ctx == nil {
		return nil, errors.New("GitHub LPK update scheduler requires a running server")
	}
	ctx, cancel := context.WithCancel(server.ctx)
	scheduler := &githubLPKUpdateScheduler{
		server: server,
		ctx:    ctx,
		cancel: cancel,
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	if !server.beginBackground() {
		cancel()
		return nil, errors.New("server is stopping")
	}
	go func() {
		defer server.endBackground()
		defer close(scheduler.done)
		scheduler.runDue(ctx)
		ticker := time.NewTicker(githubLPKUpdateScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scheduler.runDue(ctx)
			case <-scheduler.wake:
				scheduler.runDue(ctx)
			}
		}
	}()
	return scheduler, nil
}

func (s *githubLPKUpdateScheduler) notify() {
	if s == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *githubLPKUpdateScheduler) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

func (s *githubLPKUpdateScheduler) CloseContext(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.once.Do(s.Stop)
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *githubLPKUpdateScheduler) runDue(ctx context.Context) {
	now := s.server.currentTime()
	policies, err := s.server.db.GitHubLPKUpdatePolicy.Query().
		Where(
			githublpkupdatepolicy.EnabledEQ(true),
			githublpkupdatepolicy.Or(
				githublpkupdatepolicy.NextCheckAtIsNil(),
				githublpkupdatepolicy.NextCheckAtLTE(now),
			),
		).
		Order(entgo.Asc(githublpkupdatepolicy.FieldNextCheckAt), entgo.Asc(githublpkupdatepolicy.FieldID)).
		Limit(githubLPKUpdateBatchSize).
		All(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Could not query due GitHub LPK update policies", "error", err)
		}
		return
	}
	for _, policy := range policies {
		if ctx.Err() != nil {
			return
		}
		claimed, ok, err := s.server.claimGitHubLPKUpdatePolicy(ctx, policy, now)
		if err != nil {
			slog.Warn("Could not claim GitHub LPK update policy", "policy_id", policy.ID, "app_id", policy.AppID, "error", err)
			continue
		}
		if !ok {
			continue
		}
		version, updateErr := s.server.runGitHubLPKUpdate(ctx, claimed)
		if ctx.Err() != nil {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), githubLPKUpdateClaimReleaseTimeout)
			if err := s.server.releaseGitHubLPKUpdateClaim(releaseCtx, claimed, now); err != nil {
				slog.Warn("Could not release cancelled GitHub LPK update claim", "policy_id", claimed.ID, "app_id", claimed.AppID, "error", err)
			}
			cancel()
			return
		}
		if err := s.server.finishGitHubLPKUpdate(ctx, claimed, version, updateErr, now); err != nil {
			slog.Warn("Could not save GitHub LPK update result", "policy_id", claimed.ID, "app_id", claimed.AppID, "error", err)
		}
	}
}

func (s *Server) claimGitHubLPKUpdatePolicy(ctx context.Context, policy *entgo.GitHubLPKUpdatePolicy, now time.Time) (*entgo.GitHubLPKUpdatePolicy, bool, error) {
	interval := normalizedGitHubLPKUpdateInterval(policy.IntervalMinutes)
	updated, err := s.db.GitHubLPKUpdatePolicy.Update().
		Where(
			githublpkupdatepolicy.IDEQ(policy.ID),
			githublpkupdatepolicy.EnabledEQ(true),
			githublpkupdatepolicy.Or(
				githublpkupdatepolicy.NextCheckAtIsNil(),
				githublpkupdatepolicy.NextCheckAtLTE(now),
			),
		).
		SetNextCheckAt(now.Add(time.Duration(interval) * time.Minute)).
		Save(ctx)
	if err != nil || updated != 1 {
		return nil, false, err
	}
	claimed, err := s.db.GitHubLPKUpdatePolicy.Get(ctx, policy.ID)
	if err != nil {
		return nil, false, err
	}
	return claimed, true, nil
}

func (s *Server) releaseGitHubLPKUpdateClaim(ctx context.Context, claimed *entgo.GitHubLPKUpdatePolicy, retryAt time.Time) error {
	if claimed == nil || claimed.NextCheckAt == nil {
		return nil
	}
	_, err := s.db.GitHubLPKUpdatePolicy.Update().
		Where(
			githublpkupdatepolicy.IDEQ(claimed.ID),
			githublpkupdatepolicy.EnabledEQ(true),
			githublpkupdatepolicy.NextCheckAtEQ(*claimed.NextCheckAt),
		).
		SetNextCheckAt(retryAt).
		Save(ctx)
	return err
}

func (s *Server) handleUpdateGitHubLPKUpdatePolicy(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	appID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || appID <= 0 {
		badRequest(w, errors.New("invalid app id"))
		return
	}
	record, err := s.db.App.Get(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", "App not found", nil)
		return
	}
	if !isAdmin(u) && record.OwnerID != u.ID {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Only the app owner or an administrator can change automatic GitHub updates", nil)
		return
	}
	var input updateGitHubLPKUpdatePolicyRequest
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	existing, err := s.db.GitHubLPKUpdatePolicy.Query().
		Where(githublpkupdatepolicy.AppIDEQ(appID)).
		Only(r.Context())
	if err != nil && !entgo.IsNotFound(err) {
		writeError(w, http.StatusInternalServerError, "GITHUB_LPK_UPDATE_POLICY_FAILED", "Could not load automatic update settings", nil)
		return
	}
	interval := defaultGitHubLPKUpdateIntervalMinutes
	if existing != nil {
		interval = existing.IntervalMinutes
	}
	if input.IntervalMinutes != nil {
		interval = *input.IntervalMinutes
	}
	if interval < minGitHubLPKUpdateIntervalMinutes || interval > maxGitHubLPKUpdateIntervalMinutes {
		writeError(w, http.StatusBadRequest, "GITHUB_LPK_UPDATE_INTERVAL_INVALID", fmt.Sprintf("intervalMinutes must be between %d and %d", minGitHubLPKUpdateIntervalMinutes, maxGitHubLPKUpdateIntervalMinutes), nil)
		return
	}
	if input.Enabled {
		if record.Status != app.StatusAPPROVED {
			writeError(w, http.StatusUnprocessableEntity, "GITHUB_LPK_UPDATE_UNSUPPORTED", "Automatic GitHub updates require an approved app", nil)
			return
		}
		latest, latestErr := s.latestApprovedVersion(r.Context(), appID)
		if latestErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "GITHUB_LPK_UPDATE_UNSUPPORTED", "Publish a GitHub Release LPK version before enabling automatic updates", nil)
			return
		}
		if _, parseErr := parseGitHubReleaseLPKURL(latest.DownloadURL); parseErr != nil {
			writeError(w, http.StatusUnprocessableEntity, "GITHUB_LPK_UPDATE_UNSUPPORTED", parseErr.Error(), nil)
			return
		}
		if canonicalGitHubVersion(latest.Version) == "" {
			writeError(w, http.StatusUnprocessableEntity, "GITHUB_LPK_UPDATE_UNSUPPORTED", "The current published version must be valid SemVer", nil)
			return
		}
	}
	now := s.currentTime()
	scheduleCheck := input.Enabled && (existing == nil || !existing.Enabled || existing.NextCheckAt == nil)
	nextCheckAt := now
	if scheduleCheck && existing != nil && existing.LastCheckedAt != nil {
		earliest := existing.LastCheckedAt.Add(time.Duration(interval) * time.Minute)
		if earliest.After(nextCheckAt) {
			nextCheckAt = earliest
		}
	}
	var saved *entgo.GitHubLPKUpdatePolicy
	if existing == nil {
		create := s.db.GitHubLPKUpdatePolicy.Create().
			SetAppID(appID).
			SetEnabled(input.Enabled).
			SetIntervalMinutes(interval)
		if scheduleCheck {
			create.SetNextCheckAt(nextCheckAt)
		}
		saved, err = create.Save(r.Context())
	} else {
		update := existing.Update().
			SetEnabled(input.Enabled).
			SetIntervalMinutes(interval)
		if scheduleCheck {
			update.SetNextCheckAt(nextCheckAt)
		} else if !input.Enabled {
			update.ClearNextCheckAt()
		}
		saved, err = update.Save(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GITHUB_LPK_UPDATE_POLICY_FAILED", "Could not save automatic update settings", nil)
		return
	}
	if scheduleCheck && s.githubLPKUpdateScheduler != nil {
		s.githubLPKUpdateScheduler.notify()
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": toGitHubLPKUpdatePolicyDTO(saved)})
}

func (s *Server) githubLPKUpdatePolicyForApp(ctx context.Context, appID int, latest *version) *githubLPKUpdatePolicyDTO {
	if latest == nil {
		return nil
	}
	if _, err := parseGitHubReleaseLPKURL(latest.DownloadURL); err != nil {
		return nil
	}
	if canonicalGitHubVersion(latest.Version) == "" {
		return nil
	}
	policy, err := s.db.GitHubLPKUpdatePolicy.Query().
		Where(githublpkupdatepolicy.AppIDEQ(appID)).
		Only(ctx)
	if entgo.IsNotFound(err) {
		return &githubLPKUpdatePolicyDTO{IntervalMinutes: defaultGitHubLPKUpdateIntervalMinutes}
	}
	if err != nil {
		return nil
	}
	dto := toGitHubLPKUpdatePolicyDTO(policy)
	return &dto
}

func toGitHubLPKUpdatePolicyDTO(policy *entgo.GitHubLPKUpdatePolicy) githubLPKUpdatePolicyDTO {
	if policy == nil {
		return githubLPKUpdatePolicyDTO{IntervalMinutes: defaultGitHubLPKUpdateIntervalMinutes}
	}
	return githubLPKUpdatePolicyDTO{
		Enabled:         policy.Enabled,
		IntervalMinutes: policy.IntervalMinutes,
		LastCheckedAt:   policy.LastCheckedAt,
		LastSuccessAt:   policy.LastSuccessAt,
		NextCheckAt:     policy.NextCheckAt,
		LastVersion:     policy.LastVersion,
		LastError:       policy.LastError,
	}
}

func (s *Server) latestApprovedVersion(ctx context.Context, appID int) (*entgo.AppVersion, error) {
	return s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(appID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(orderVersionsBySoftwareUpdate(), entgo.Desc(appversion.FieldCreatedAt), entgo.Desc(appversion.FieldID)).
		First(ctx)
}

func (s *Server) runGitHubLPKUpdate(ctx context.Context, policy *entgo.GitHubLPKUpdatePolicy) (string, error) {
	activePolicy, err := s.db.GitHubLPKUpdatePolicy.Get(ctx, policy.ID)
	if err != nil {
		return "", err
	}
	if !activePolicy.Enabled {
		return "", errGitHubLPKUpdateDisabled
	}
	policy = activePolicy
	record, err := s.db.App.Get(ctx, policy.AppID)
	if err != nil {
		return "", err
	}
	if record.Status != app.StatusAPPROVED {
		return "", errors.New("automatic GitHub updates require an approved app")
	}
	owner, err := s.db.User.Get(ctx, record.OwnerID)
	if err != nil {
		return "", errors.New("app owner is unavailable for automatic version publishing")
	}
	if owner.Disabled {
		return "", errors.New("app owner is disabled; automatic version publishing is paused")
	}
	current, err := s.latestApprovedVersion(ctx, record.ID)
	if err != nil {
		return "", errGitHubLPKUpdateUnsupported
	}
	if canonicalGitHubVersion(current.Version) == "" {
		return "", errors.New("the current published version is not valid SemVer")
	}
	upstream, err := parseGitHubReleaseLPKURL(current.DownloadURL)
	if err != nil {
		return "", errGitHubLPKUpdateUnsupported
	}
	release, _, err := s.githubReleases.GetLatestRelease(ctx, upstream.Owner, upstream.Repo)
	if err != nil {
		return "", fmt.Errorf("GitHub latest release request failed: %w", err)
	}
	if release == nil || release.GetDraft() || release.GetPrerelease() {
		return "", errors.New("GitHub latest release is unavailable")
	}
	targetVersion := githubReleaseVersion(release.GetTagName())
	if targetVersion == "" {
		return "", errors.New("GitHub latest release has no usable tag version")
	}
	asset, err := selectGitHubLPKReleaseAsset(release.Assets, upstream, record.PackageID, current.Version, release.GetTagName(), targetVersion)
	if err != nil {
		return "", err
	}
	sha256, err := githubReleaseAssetSHA256(asset.GetDigest())
	if err != nil {
		return "", fmt.Errorf("GitHub release asset %q: %w", asset.GetName(), err)
	}
	downloadURL := strings.TrimSpace(asset.GetBrowserDownloadURL())
	selected, err := parseGitHubReleaseLPKURL(downloadURL)
	if err != nil || !strings.EqualFold(selected.Owner, upstream.Owner) || !strings.EqualFold(selected.Repo, upstream.Repo) {
		return "", errors.New("GitHub release asset has an invalid browser download URL")
	}
	if !strings.EqualFold(selected.AssetName, asset.GetName()) {
		return "", errors.New("GitHub release asset name does not match its browser download URL")
	}
	if selected.Tag != release.GetTagName() {
		return "", errors.New("GitHub release asset tag does not match the latest release")
	}
	if asset.GetSize() <= 0 {
		return "", errors.New("GitHub release asset has no valid file size")
	}
	if state := strings.TrimSpace(asset.GetState()); state != "" && !strings.EqualFold(state, "uploaded") {
		return "", fmt.Errorf("GitHub release asset is not uploaded (state %q)", state)
	}
	comparison, comparable := compareGitHubReleaseVersions(targetVersion, current.Version)
	if !comparable {
		return "", errors.New("the current published version is not valid SemVer")
	}
	if comparison < 0 {
		return current.Version, nil
	}
	var upstreamPublishedAt *time.Time
	if publishedAt := release.GetPublishedAt().Time; !publishedAt.IsZero() {
		upstreamPublishedAt = &publishedAt
	} else if createdAt := release.GetCreatedAt().Time; !createdAt.IsZero() {
		upstreamPublishedAt = &createdAt
	}
	input := automaticGitHubVersionInput{
		Version:             targetVersion,
		Changelog:           strings.TrimSpace(release.GetBody()),
		DownloadURL:         downloadURL,
		SHA256:              sha256,
		FileSize:            int64(asset.GetSize()),
		UpstreamPublishedAt: upstreamPublishedAt,
	}
	if automaticGitHubVersionMatches(current, input) {
		return targetVersion, nil
	}
	activePolicy, err = s.db.GitHubLPKUpdatePolicy.Get(ctx, policy.ID)
	if err != nil {
		return "", err
	}
	if !activePolicy.Enabled {
		return "", errGitHubLPKUpdateDisabled
	}
	freshRecord, err := s.db.App.Get(ctx, record.ID)
	if err != nil {
		return "", err
	}
	if freshRecord.Status != app.StatusAPPROVED {
		return "", errors.New("automatic GitHub updates require an approved app")
	}
	freshCurrent, err := s.latestApprovedVersion(ctx, record.ID)
	if err != nil {
		return "", errGitHubLPKUpdateUnsupported
	}
	if freshCurrent.ID != current.ID || freshCurrent.DownloadURL != current.DownloadURL || freshCurrent.Version != current.Version {
		return "", errors.New("the latest published version changed during the GitHub check; retrying later")
	}
	owner, err = s.db.User.Get(ctx, freshRecord.OwnerID)
	if err != nil {
		return "", errors.New("app owner is unavailable for automatic version publishing")
	}
	if owner.Disabled {
		return "", errors.New("app owner is disabled; automatic version publishing is paused")
	}
	if _, err := s.upsertAutomaticGitHubVersion(ctx, freshRecord, owner, input); err != nil {
		return "", err
	}
	return targetVersion, nil
}

func (s *Server) finishGitHubLPKUpdate(ctx context.Context, policy *entgo.GitHubLPKUpdatePolicy, version string, updateErr error, checkedAt time.Time) error {
	for range 3 {
		current, err := s.db.GitHubLPKUpdatePolicy.Get(ctx, policy.ID)
		if err != nil {
			return err
		}
		update := s.db.GitHubLPKUpdatePolicy.Update().
			Where(
				githublpkupdatepolicy.IDEQ(current.ID),
				githublpkupdatepolicy.EnabledEQ(current.Enabled),
				githublpkupdatepolicy.IntervalMinutesEQ(current.IntervalMinutes),
			).
			SetLastCheckedAt(checkedAt)
		if current.NextCheckAt == nil {
			update.Where(githublpkupdatepolicy.NextCheckAtIsNil())
		} else {
			update.Where(githublpkupdatepolicy.NextCheckAtEQ(*current.NextCheckAt))
		}
		if current.Enabled {
			interval := normalizedGitHubLPKUpdateInterval(current.IntervalMinutes)
			update.SetNextCheckAt(checkedAt.Add(time.Duration(interval) * time.Minute))
		} else {
			update.ClearNextCheckAt()
		}
		if !errors.Is(updateErr, errGitHubLPKUpdateDisabled) {
			if updateErr == nil {
				update.SetLastSuccessAt(checkedAt).
					SetLastVersion(version).
					SetLastError("")
			} else {
				message := strings.TrimSpace(updateErr.Error())
				if len(message) > 2000 {
					message = message[:2000]
				}
				update.SetLastError(message)
			}
		}
		updated, err := update.Save(ctx)
		if err != nil {
			return err
		}
		if updated == 1 {
			return nil
		}
	}
	return errors.New("automatic GitHub update settings changed concurrently")
}

func (s *Server) upsertAutomaticGitHubVersion(ctx context.Context, record *entgo.App, owner *entgo.User, input automaticGitHubVersionInput) (*entgo.AppVersion, error) {
	direct := isAdmin(owner) || record.AllowUnreviewedUpdates
	status := appversion.StatusPENDING
	if direct {
		status = appversion.StatusAPPROVED
	}
	existing, err := s.db.AppVersion.Query().
		Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ(input.Version)).
		Only(ctx)
	if err != nil && !entgo.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		if existing.SourceType != appversion.SourceTypeGITHUB || !sameGitHubReleaseAsset(existing.DownloadURL, input.DownloadURL) {
			return nil, fmt.Errorf("version %s already exists from a different source; manual review is required", input.Version)
		}
		matches := automaticGitHubVersionMatches(existing, input)
		switch existing.Status {
		case appversion.StatusAPPROVED:
			if matches {
				return existing, nil
			}
			if !direct {
				return nil, fmt.Errorf("GitHub release %s changed an already approved version; manual review is required", input.Version)
			}
		case appversion.StatusPENDING:
			if !matches {
				return nil, fmt.Errorf("GitHub release %s changed while its version is pending review; manual review is required", input.Version)
			}
			if err := s.ensureAutomaticVersionReview(ctx, record.ID, existing.ID, owner.ID); err != nil {
				return nil, err
			}
			return existing, nil
		case appversion.StatusREJECTED:
			return nil, fmt.Errorf("GitHub release %s was rejected; manual action is required before retrying", input.Version)
		}
		update := s.db.AppVersion.Update().
			Where(
				appversion.IDEQ(existing.ID),
				appversion.StatusEQ(existing.Status),
			).
			SetUploaderID(owner.ID).
			SetChangelog(input.Changelog).
			SetStatus(status).
			SetSourceType(appversion.SourceTypeGITHUB).
			SetDownloadURL(input.DownloadURL).
			SetSha256(input.SHA256).
			SetFileSize(input.FileSize).
			SetNillableUpstreamPublishedAt(input.UpstreamPublishedAt)
		if status == appversion.StatusAPPROVED && (existing.Status != appversion.StatusAPPROVED || existing.PublishedAt == nil) {
			update.SetPublishedAt(s.currentTime())
		}
		updatedCount, err := update.Save(ctx)
		if err != nil {
			return nil, err
		}
		if updatedCount != 1 {
			return nil, fmt.Errorf("version %s changed concurrently; retrying later", input.Version)
		}
		updated, err := s.db.AppVersion.Get(ctx, existing.ID)
		if err != nil {
			return nil, err
		}
		if status == appversion.StatusPENDING {
			if err := s.ensureAutomaticVersionReview(ctx, record.ID, updated.ID, owner.ID); err != nil {
				return nil, err
			}
		} else {
			s.afterAutomaticGitHubVersionApproved(ctx, record.ID)
		}
		return updated, nil
	}
	create := s.db.AppVersion.Create().
		SetAppID(record.ID).
		SetUploaderID(owner.ID).
		SetVersion(input.Version).
		SetChangelog(input.Changelog).
		SetStatus(status).
		SetSourceType(appversion.SourceTypeGITHUB).
		SetDownloadURL(input.DownloadURL).
		SetSha256(input.SHA256).
		SetFileSize(input.FileSize).
		SetNillableUpstreamPublishedAt(input.UpstreamPublishedAt)
	if status == appversion.StatusAPPROVED {
		create.SetPublishedAt(s.currentTime())
	}
	created, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	if status == appversion.StatusPENDING {
		if err := s.ensureAutomaticVersionReview(ctx, record.ID, created.ID, owner.ID); err != nil {
			return nil, err
		}
	} else {
		s.afterAutomaticGitHubVersionApproved(ctx, record.ID)
	}
	return created, nil
}

func (s *Server) ensureAutomaticVersionReview(ctx context.Context, appID, versionID, requesterID int) error {
	exists, err := s.db.ReviewRequest.Query().
		Where(
			reviewrequest.KindEQ(reviewrequest.KindVERSION_UPLOAD),
			reviewrequest.StatusEQ(reviewrequest.StatusPENDING),
			reviewrequest.VersionIDEQ(versionID),
		).
		Exist(ctx)
	if err != nil || exists {
		return err
	}
	_, err = s.db.ReviewRequest.Create().
		SetKind(reviewrequest.KindVERSION_UPLOAD).
		SetStatus(reviewrequest.StatusPENDING).
		SetAppID(appID).
		SetVersionID(versionID).
		SetRequesterID(requesterID).
		SetNote("Automatically discovered from the latest GitHub Release").
		Save(ctx)
	return err
}

func (s *Server) afterAutomaticGitHubVersionApproved(ctx context.Context, appID int) {
	s.clearAppOutdatedMarksContext(ctx, appID)
	if _, _, err := s.enforceVersionRetention(ctx, appID); err != nil {
		slog.Warn("Could not enforce version retention after automatic GitHub update", "app_id", appID, "error", err)
	}
	s.invalidateSourceFeed()
}

func parseGitHubReleaseLPKURL(rawURL string) (githubReleaseLPK, error) {
	value := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return githubReleaseLPK{}, errors.New("a standard HTTPS GitHub Release LPK URL is required")
	}
	escapedParts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(escapedParts) != 6 || escapedParts[2] != "releases" || escapedParts[3] != "download" {
		return githubReleaseLPK{}, errors.New("a GitHub /owner/repo/releases/download/tag/file.lpk URL is required")
	}
	parts := make([]string, len(escapedParts))
	for i, part := range escapedParts {
		decoded, decodeErr := url.PathUnescape(part)
		if decodeErr != nil || strings.TrimSpace(decoded) == "" || strings.Contains(decoded, "/") {
			return githubReleaseLPK{}, errors.New("the GitHub Release LPK URL contains an invalid path segment")
		}
		parts[i] = decoded
	}
	if !strings.HasSuffix(strings.ToLower(parts[5]), ".lpk") {
		return githubReleaseLPK{}, errors.New("the GitHub Release asset must be an .lpk file")
	}
	return githubReleaseLPK{
		Owner:     parts[0],
		Repo:      parts[1],
		Tag:       parts[4],
		AssetName: parts[5],
		URL:       value,
	}, nil
}

func selectGitHubLPKReleaseAsset(assets []*github.ReleaseAsset, current githubReleaseLPK, packageID, currentVersion, targetTag, targetVersion string) (*github.ReleaseAsset, error) {
	lpkAssets := make([]*github.ReleaseAsset, 0, len(assets))
	for _, asset := range assets {
		if asset != nil && strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.GetName())), ".lpk") {
			lpkAssets = append(lpkAssets, asset)
		}
	}
	if len(lpkAssets) == 0 {
		return nil, errors.New("GitHub latest release has no LPK asset")
	}
	expectedNames := make([]string, 0, 4)
	for _, replacement := range [][2]string{
		{current.Tag, targetTag},
		{currentVersion, targetVersion},
		{"v" + strings.TrimPrefix(currentVersion, "v"), "v" + strings.TrimPrefix(targetVersion, "v")},
	} {
		if replacement[0] != "" && replacement[1] != "" && strings.Contains(current.AssetName, replacement[0]) {
			expectedNames = append(expectedNames, strings.ReplaceAll(current.AssetName, replacement[0], replacement[1]))
		}
	}
	expectedNames = append(expectedNames, current.AssetName)
	for _, expected := range expectedNames {
		matches := matchingGitHubAssets(lpkAssets, func(asset *github.ReleaseAsset) bool {
			return strings.EqualFold(asset.GetName(), expected)
		})
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	if packageID = strings.ToLower(strings.TrimSpace(packageID)); packageID != "" {
		matches := matchingGitHubAssets(lpkAssets, func(asset *github.ReleaseAsset) bool {
			return strings.Contains(strings.ToLower(asset.GetName()), packageID)
		})
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	if len(lpkAssets) == 1 {
		return lpkAssets[0], nil
	}
	return nil, fmt.Errorf("GitHub latest release has %d LPK assets and no unique match", len(lpkAssets))
}

func matchingGitHubAssets(assets []*github.ReleaseAsset, match func(*github.ReleaseAsset) bool) []*github.ReleaseAsset {
	out := make([]*github.ReleaseAsset, 0, len(assets))
	for _, asset := range assets {
		if match(asset) {
			out = append(out, asset)
		}
	}
	return out
}

func githubReleaseAssetSHA256(digest string) (string, error) {
	algorithm, value, ok := strings.Cut(strings.ToLower(strings.TrimSpace(digest)), ":")
	if !ok || algorithm != "sha256" || !isSHA256Hex(value) {
		return "", errors.New("asset digest is not a valid SHA256")
	}
	return value, nil
}

func githubReleaseVersion(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	match := releaseVersionPattern.FindStringSubmatch(tag)
	if len(match) != 2 {
		return ""
	}
	canonical := canonicalGitHubVersion(match[1])
	if canonical == "" {
		return ""
	}
	return strings.TrimPrefix(canonical, "v")
}

func compareGitHubReleaseVersions(left, right string) (int, bool) {
	leftCanonical := canonicalGitHubVersion(left)
	rightCanonical := canonicalGitHubVersion(right)
	if leftCanonical == "" || rightCanonical == "" {
		return 0, false
	}
	return semver.Compare(leftCanonical, rightCanonical), true
}

func canonicalGitHubVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if value[0] != 'v' && value[0] != 'V' {
		value = "v" + value
	} else if value[0] == 'V' {
		value = "v" + value[1:]
	}
	if !semver.IsValid(value) {
		return ""
	}
	return value
}

func automaticGitHubVersionMatches(record *entgo.AppVersion, input automaticGitHubVersionInput) bool {
	if record == nil {
		return false
	}
	return record.Version == input.Version &&
		strings.TrimSpace(record.Changelog) == strings.TrimSpace(input.Changelog) &&
		strings.EqualFold(strings.TrimSpace(record.DownloadURL), strings.TrimSpace(input.DownloadURL)) &&
		strings.EqualFold(strings.TrimSpace(record.Sha256), strings.TrimSpace(input.SHA256)) &&
		record.FileSize == input.FileSize &&
		timesEqual(record.UpstreamPublishedAt, input.UpstreamPublishedAt) &&
		record.SourceType == appversion.SourceTypeGITHUB
}

func timesEqual(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func sameGitHubReleaseAsset(left, right string) bool {
	leftRelease, leftErr := parseGitHubReleaseLPKURL(left)
	rightRelease, rightErr := parseGitHubReleaseLPKURL(right)
	return leftErr == nil && rightErr == nil &&
		strings.EqualFold(leftRelease.Owner, rightRelease.Owner) &&
		strings.EqualFold(leftRelease.Repo, rightRelease.Repo) &&
		strings.EqualFold(leftRelease.AssetName, rightRelease.AssetName)
}

func normalizedGitHubLPKUpdateInterval(value int) int {
	if value < minGitHubLPKUpdateIntervalMinutes || value > maxGitHubLPKUpdateIntervalMinutes {
		return defaultGitHubLPKUpdateIntervalMinutes
	}
	return value
}
