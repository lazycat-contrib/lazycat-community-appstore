package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/reviewrequest"
)

const versionStorageCleanupTimeout = 5 * time.Second

type versionRetentionPolicy struct {
	Mode                 string `json:"mode"`
	SiteMaxVersions      int    `json:"siteMaxVersions"`
	AppMaxVersions       *int   `json:"appMaxVersions,omitzero"`
	EffectiveMaxVersions int    `json:"effectiveMaxVersions"`
}

type versionCleanupWarning struct {
	VersionID   int    `json:"versionId"`
	StorageKey  string `json:"storageKey"`
	StoragePath string `json:"storagePath"`
	Message     string `json:"message"`
}

type deletedVersionResult struct {
	Version version                `json:"version"`
	Warning *versionCleanupWarning `json:"cleanupWarning,omitzero"`
}

func (s *Server) versionRetentionPolicyForApp(ctx context.Context, record *entgo.App) versionRetentionPolicy {
	return versionRetentionPolicyFromValues(record.VersionRetentionCount, s.effectiveMaxVersions(ctx))
}

func versionRetentionPolicyFromValues(appMaxVersions *int, siteMaxVersions int) versionRetentionPolicy {
	policy := versionRetentionPolicy{
		Mode:                 "INHERIT",
		SiteMaxVersions:      siteMaxVersions,
		EffectiveMaxVersions: siteMaxVersions,
	}
	if appMaxVersions != nil {
		policy.Mode = "CUSTOM"
		policy.AppMaxVersions = appMaxVersions
		policy.EffectiveMaxVersions = *appMaxVersions
	}
	return policy
}

func (s *Server) updateAppVersionRetention(ctx context.Context, appID int, maxVersions *int) (versionRetentionPolicy, []deletedVersionResult, error) {
	siteMaxVersions := s.effectiveMaxVersions(ctx)
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	update := tx.App.UpdateOneID(appID)
	if maxVersions == nil {
		update.ClearVersionRetentionCount()
	} else {
		update.SetVersionRetentionCount(*maxVersions)
	}
	record, err := update.Save(ctx)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	policy := versionRetentionPolicyFromValues(record.VersionRetentionCount, siteMaxVersions)
	deleted, err := pruneApprovedVersions(ctx, tx, appID, policy.EffectiveMaxVersions)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	for index := range deleted {
		deleted[index] = s.cleanupDeletedVersionResult(ctx, deleted[index])
	}
	return policy, deleted, nil
}

func (s *Server) enforceVersionRetention(ctx context.Context, appID int) (versionRetentionPolicy, []deletedVersionResult, error) {
	siteMaxVersions := s.effectiveMaxVersions(ctx)
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := tx.App.Get(ctx, appID)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	policy := versionRetentionPolicyFromValues(record.VersionRetentionCount, siteMaxVersions)
	deleted, err := pruneApprovedVersions(ctx, tx, appID, policy.EffectiveMaxVersions)
	if err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return versionRetentionPolicy{}, nil, err
	}
	for index := range deleted {
		deleted[index] = s.cleanupDeletedVersionResult(ctx, deleted[index])
	}
	return policy, deleted, nil
}

func pruneApprovedVersions(ctx context.Context, tx *entgo.Tx, appID, maxVersions int) ([]deletedVersionResult, error) {
	if maxVersions == 0 {
		return nil, nil
	}
	records, err := tx.AppVersion.Query().
		Where(appversion.AppIDEQ(appID), appversion.StatusEQ(appversion.StatusAPPROVED)).
		Order(
			entgo.Desc(appversion.FieldPublishedAt),
			entgo.Desc(appversion.FieldCreatedAt),
			entgo.Desc(appversion.FieldID),
		).
		All(ctx)
	if err != nil || len(records) <= maxVersions {
		return nil, err
	}
	deleted := make([]deletedVersionResult, 0, len(records)-maxVersions)
	for _, record := range records[maxVersions:] {
		if _, err := tx.ReviewRequest.Update().
			Where(reviewrequest.VersionIDEQ(record.ID)).
			ClearVersionID().
			Save(ctx); err != nil {
			return nil, err
		}
		if err := tx.AppVersion.DeleteOneID(record.ID).Exec(ctx); err != nil {
			return nil, err
		}
		deleted = append(deleted, deletedVersionResult{Version: toVersionDTO(record)})
	}
	return deleted, nil
}

func (s *Server) deleteAppVersion(ctx context.Context, appID, versionID int) (deletedVersionResult, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return deletedVersionResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := tx.AppVersion.Query().
		Where(appversion.IDEQ(versionID), appversion.AppIDEQ(appID)).
		Only(ctx)
	if err != nil {
		return deletedVersionResult{}, err
	}
	if _, err := tx.ReviewRequest.Update().
		Where(reviewrequest.VersionIDEQ(record.ID)).
		ClearVersionID().
		Save(ctx); err != nil {
		return deletedVersionResult{}, err
	}
	if err := tx.AppVersion.DeleteOneID(record.ID).Exec(ctx); err != nil {
		return deletedVersionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return deletedVersionResult{}, err
	}
	return s.cleanupDeletedVersion(ctx, record), nil
}

func (s *Server) cleanupDeletedVersion(ctx context.Context, record *entgo.AppVersion) deletedVersionResult {
	return s.cleanupDeletedVersionResult(ctx, deletedVersionResult{Version: toVersionDTO(record)})
}

func (s *Server) cleanupDeletedVersionResult(ctx context.Context, result deletedVersionResult) deletedVersionResult {
	if strings.TrimSpace(result.Version.StoragePath) == "" {
		return result
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), versionStorageCleanupTimeout)
	defer cancel()
	if err := s.deleteStoredObjectChecked(cleanupCtx, result.Version.StorageKey, result.Version.StoragePath); err != nil {
		slog.Warn("Could not delete stored app version object",
			"app_id", result.Version.AppID,
			"version_id", result.Version.ID,
			"storage_key", result.Version.StorageKey,
			"storage_path", result.Version.StoragePath,
			"error", err,
		)
		result.Warning = &versionCleanupWarning{
			VersionID:   result.Version.ID,
			StorageKey:  result.Version.StorageKey,
			StoragePath: result.Version.StoragePath,
			Message:     "Stored package cleanup failed",
		}
	}
	return result
}
