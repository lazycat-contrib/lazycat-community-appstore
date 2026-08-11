package clientserver

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsetting"
	"lazycat.community/appstore/ent/clientsource"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/mirrorprobe"
)

const (
	settingMirrorBenchmarkEnabled         = "github_mirror_benchmark_enabled"
	settingMirrorBenchmarkIntervalMinutes = "github_mirror_benchmark_interval_minutes"
	settingMirrorBenchmarkLastRunAt       = "github_mirror_benchmark_last_run_at"
	settingMirrorBenchmarkLastStatus      = "github_mirror_benchmark_last_status"
	settingMirrorBenchmarkSnapshot        = "github_mirror_benchmark_snapshot_v1"

	defaultMirrorBenchmarkIntervalMinutes = 6 * 60
	minMirrorBenchmarkIntervalMinutes     = 30
	maxMirrorBenchmarkIntervalMinutes     = 24 * 60
	maxMirrorBenchmarkSamples             = 8
	maxMirrorBenchmarkTargetsPerKind      = 32
	maxMirrorBenchmarkSnapshotResults     = 256
)

type mirrorBenchmarkSample struct {
	At             time.Time `json:"at"`
	Successful     bool      `json:"successful"`
	BytesPerSecond int64     `json:"bytesPerSecond,omitempty"`
}

type mirrorBenchmarkResult struct {
	ID                  string                  `json:"id"`
	Kind                string                  `json:"kind"`
	Samples             []mirrorBenchmarkSample `json:"samples"`
	SpeedBytesPerSecond int64                   `json:"speedBytesPerSecond,omitempty"`
	StabilityPercent    int                     `json:"stabilityPercent,omitempty"`
	Score               int64                   `json:"score,omitempty"`
	Status              string                  `json:"status"`
	UpdatedAt           time.Time               `json:"updatedAt"`
}

type mirrorBenchmarkSnapshot struct {
	Version     int                              `json:"version"`
	EvaluatedAt time.Time                        `json:"evaluatedAt"`
	Results     map[string]mirrorBenchmarkResult `json:"results"`
}

type mirrorBenchmarkRunResult struct {
	Tested    int `json:"tested"`
	Available int `json:"available"`
}

func sanitizeMirrorBenchmarkInterval(value int) int {
	if value <= 0 {
		return defaultMirrorBenchmarkIntervalMinutes
	}
	if value < minMirrorBenchmarkIntervalMinutes {
		return minMirrorBenchmarkIntervalMinutes
	}
	if value > maxMirrorBenchmarkIntervalMinutes {
		return maxMirrorBenchmarkIntervalMinutes
	}
	return value
}

func (s *Server) mirrorBenchmarkConfig(ctx context.Context, userID string) (bool, int, *time.Time, string) {
	enabled, _ := strconv.ParseBool(strings.TrimSpace(s.clientSetting(ctx, userID, settingMirrorBenchmarkEnabled)))
	interval, _ := strconv.Atoi(strings.TrimSpace(s.clientSetting(ctx, userID, settingMirrorBenchmarkIntervalMinutes)))
	interval = sanitizeMirrorBenchmarkInterval(interval)
	var lastRun *time.Time
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s.clientSetting(ctx, userID, settingMirrorBenchmarkLastRunAt))); err == nil {
		parsed = parsed.UTC()
		lastRun = &parsed
	}
	return enabled, interval, lastRun, strings.TrimSpace(s.clientSetting(ctx, userID, settingMirrorBenchmarkLastStatus))
}

func mirrorBenchmarkDue(enabled bool, intervalMinutes int, lastRun *time.Time, now time.Time) bool {
	if !enabled {
		return false
	}
	if lastRun == nil {
		return true
	}
	return !lastRun.Add(time.Duration(sanitizeMirrorBenchmarkInterval(intervalMinutes)) * time.Minute).After(now)
}

func mirrorBenchmarkSchedule(enabled bool, intervalMinutes int, lastRun *time.Time, now time.Time) (string, *time.Time) {
	if !enabled {
		return "paused", nil
	}
	if lastRun == nil {
		return "due", nil
	}
	next := lastRun.Add(time.Duration(sanitizeMirrorBenchmarkInterval(intervalMinutes)) * time.Minute).UTC()
	if !next.After(now) {
		return "due", nil
	}
	return "scheduled", &next
}

func (s *Server) loadMirrorBenchmarkSnapshot(ctx context.Context, userID string) mirrorBenchmarkSnapshot {
	snapshot := mirrorBenchmarkSnapshot{Version: 1, Results: map[string]mirrorBenchmarkResult{}}
	raw := strings.TrimSpace(s.clientSetting(ctx, userID, settingMirrorBenchmarkSnapshot))
	if raw == "" || json.Unmarshal([]byte(raw), &snapshot) != nil || snapshot.Version != 1 {
		return mirrorBenchmarkSnapshot{Version: 1, Results: map[string]mirrorBenchmarkResult{}}
	}
	if snapshot.Results == nil {
		snapshot.Results = map[string]mirrorBenchmarkResult{}
	}
	return snapshot
}

func (s *Server) saveMirrorBenchmarkSnapshot(ctx context.Context, userID string, snapshot mirrorBenchmarkSnapshot) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return s.setClientSettingValue(ctx, userID, settingMirrorBenchmarkSnapshot, string(raw))
}

func (s *Server) setClientSettingValue(ctx context.Context, userID, key, value string) error {
	return setClientSettingValue(ctx, s.db, userID, key, value)
}

func setClientSettingValue(ctx context.Context, db *ent.Client, userID, key, value string) error {
	record, err := db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(userID), clientsetting.KeyEQ(key)).
		Only(ctx)
	if err == nil {
		_, err = db.ClientSetting.UpdateOneID(record.ID).SetValue(value).Save(ctx)
		return err
	}
	if !ent.IsNotFound(err) {
		return err
	}
	_, err = db.ClientSetting.Create().SetUserID(userID).SetKey(key).SetValue(value).Save(ctx)
	if !ent.IsConstraintError(err) {
		return err
	}
	record, err = db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(userID), clientsetting.KeyEQ(key)).
		Only(ctx)
	if err != nil {
		return err
	}
	_, err = db.ClientSetting.UpdateOneID(record.ID).SetValue(value).Save(ctx)
	return err
}

func (s *Server) runMirrorBenchmark(ctx context.Context, userID string) (mirrorBenchmarkRunResult, error) {
	defer s.lockMirrorBenchmark(userID)()

	entriesByKind, upstreamByKind, currentIDs, err := s.collectMirrorBenchmarkInputs(ctx, userID)
	if err != nil {
		return mirrorBenchmarkRunResult{}, err
	}
	measurements := make([]mirrorprobe.Measurement, 0)
	for _, kind := range []string{mirror.KindDownload, mirror.KindRaw} {
		upstream := upstreamByKind[kind]
		if upstream == "" {
			continue
		}
		measurements = append(measurements, mirrorprobe.Measure(ctx, entriesByKind[kind], upstream, s.mirrorProbe)...)
	}
	now := time.Now().UTC()
	available := 0
	for _, measurement := range measurements {
		if measurement.Successful {
			available++
		}
	}
	status := "success"
	if available == 0 {
		status = "failed"
	} else if available < len(measurements) {
		status = "partial"
	}
	snapshot := s.loadMirrorBenchmarkSnapshot(ctx, userID)
	snapshot = mergeMirrorBenchmarkMeasurements(snapshot, measurements, currentIDs, now)
	if err := s.saveMirrorBenchmarkRun(ctx, userID, snapshot, now, status); err != nil {
		return mirrorBenchmarkRunResult{}, err
	}
	return mirrorBenchmarkRunResult{Tested: len(measurements), Available: available}, nil
}

func (s *Server) saveMirrorBenchmarkRun(ctx context.Context, userID string, snapshot mirrorBenchmarkSnapshot, now time.Time, status string) error {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	for key, value := range map[string]string{
		settingMirrorBenchmarkSnapshot:   string(raw),
		settingMirrorBenchmarkLastRunAt:  now.Format(time.RFC3339Nano),
		settingMirrorBenchmarkLastStatus: status,
	} {
		if err := setClientSettingValue(ctx, client, userID, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Server) collectMirrorBenchmarkInputs(ctx context.Context, userID string) (map[string][]mirror.Entry, map[string]string, map[string]struct{}, error) {
	sources, err := s.db.ClientSource.Query().
		Where(clientsource.UserIDEQ(userID)).
		Order(ent.Asc(clientsource.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	apps, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.HasSourceWith(clientsource.UserIDEQ(userID))).
		Order(ent.Asc(clientsourceapp.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	upstreamByKind := map[string]string{}
	for _, app := range apps {
		var version VersionDTO
		if json.Unmarshal([]byte(app.LatestVersionJSON), &version) != nil {
			continue
		}
		upstream := strings.TrimSpace(version.UpstreamDownloadURL)
		if upstream == "" {
			upstream = strings.TrimSpace(version.DownloadURL)
		}
		kind := mirror.KindForURL(upstream)
		if kind != "" && upstreamByKind[kind] == "" {
			upstreamByKind[kind] = upstream
		}
	}
	entriesByKind := map[string][]mirror.Entry{mirror.KindDownload: {}, mirror.KindRaw: {}}
	currentIDs := map[string]struct{}{}
	for _, source := range sources {
		for _, entry := range sourceMirrors(source) {
			if _, seen := currentIDs[entry.ID]; seen {
				continue
			}
			if len(entriesByKind[entry.Kind]) >= maxMirrorBenchmarkTargetsPerKind {
				continue
			}
			currentIDs[entry.ID] = struct{}{}
			entriesByKind[entry.Kind] = append(entriesByKind[entry.Kind], entry)
		}
	}
	return entriesByKind, upstreamByKind, currentIDs, nil
}

func mergeMirrorBenchmarkMeasurements(snapshot mirrorBenchmarkSnapshot, measurements []mirrorprobe.Measurement, currentIDs map[string]struct{}, now time.Time) mirrorBenchmarkSnapshot {
	if snapshot.Results == nil {
		snapshot.Results = map[string]mirrorBenchmarkResult{}
	}
	if currentIDs != nil {
		for id := range snapshot.Results {
			if _, ok := currentIDs[id]; !ok {
				delete(snapshot.Results, id)
			}
		}
	}
	for _, measurement := range measurements {
		result := snapshot.Results[measurement.Entry.ID]
		result.ID = measurement.Entry.ID
		result.Kind = measurement.Entry.Kind
		result.Samples = append(result.Samples, mirrorBenchmarkSample{
			At:             now,
			Successful:     measurement.Successful,
			BytesPerSecond: measurement.BytesPerSecond,
		})
		if len(result.Samples) > maxMirrorBenchmarkSamples {
			result.Samples = slices.Clone(result.Samples[len(result.Samples)-maxMirrorBenchmarkSamples:])
		}
		recalculateMirrorBenchmarkResult(&result, now)
		snapshot.Results[result.ID] = result
	}
	trimMirrorBenchmarkSnapshot(&snapshot)
	snapshot.Version = 1
	snapshot.EvaluatedAt = now
	return snapshot
}

func trimMirrorBenchmarkSnapshot(snapshot *mirrorBenchmarkSnapshot) {
	if snapshot == nil || len(snapshot.Results) <= maxMirrorBenchmarkSnapshotResults {
		return
	}
	ids := make([]string, 0, len(snapshot.Results))
	for id := range snapshot.Results {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(left, right string) int {
		leftUpdated := snapshot.Results[left].UpdatedAt
		rightUpdated := snapshot.Results[right].UpdatedAt
		if leftUpdated.Before(rightUpdated) {
			return -1
		}
		if leftUpdated.After(rightUpdated) {
			return 1
		}
		return strings.Compare(left, right)
	})
	for _, id := range ids[:len(ids)-maxMirrorBenchmarkSnapshotResults] {
		delete(snapshot.Results, id)
	}
}

func recalculateMirrorBenchmarkResult(result *mirrorBenchmarkResult, now time.Time) {
	if result == nil || len(result.Samples) == 0 {
		return
	}
	successes := 0
	var ewma float64
	for _, sample := range result.Samples {
		if !sample.Successful || sample.BytesPerSecond <= 0 {
			continue
		}
		successes++
		if ewma == 0 {
			ewma = float64(sample.BytesPerSecond)
		} else {
			ewma = 0.35*float64(sample.BytesPerSecond) + 0.65*ewma
		}
	}
	result.SpeedBytesPerSecond = int64(ewma)
	result.StabilityPercent = successes * 100 / len(result.Samples)
	result.Score = result.SpeedBytesPerSecond * int64(result.StabilityPercent) / 100
	last := result.Samples[len(result.Samples)-1]
	switch {
	case !last.Successful:
		result.Status = "unavailable"
		result.Score = 0
	case result.StabilityPercent < 75:
		result.Status = "unstable"
		result.Score /= 2
	default:
		result.Status = "healthy"
	}
	result.UpdatedAt = now
}

func applyMirrorBenchmarkSnapshot(entries []mirror.Entry, snapshot mirrorBenchmarkSnapshot) []mirror.Entry {
	out := make([]mirror.Entry, len(entries))
	for index, entry := range entries {
		if result, ok := snapshot.Results[entry.ID]; ok {
			entry.BenchmarkStatus = result.Status
			entry.SpeedBytesPerSecond = result.SpeedBytesPerSecond
			entry.StabilityPercent = result.StabilityPercent
			entry.BenchmarkScore = result.Score
			updatedAt := result.UpdatedAt
			entry.LastBenchmarkAt = &updatedAt
		}
		out[index] = entry
	}
	return out
}

func (s *Server) preferredMirrorForURL(ctx context.Context, userID string, entries []mirror.Entry, upstreamURL, tiePreferredID string) (mirror.Entry, bool) {
	ordered := mirror.OrderedForURL(entries, upstreamURL, tiePreferredID)
	if len(ordered) == 0 {
		return mirror.Entry{}, false
	}
	_, interval, _, _ := s.mirrorBenchmarkConfig(ctx, userID)
	freshAfter := time.Now().Add(-time.Duration(interval) * time.Minute)
	snapshot := s.loadMirrorBenchmarkSnapshot(ctx, userID)
	if !mirrorBenchmarkCoverageFresh(snapshot, ordered, freshAfter) {
		unlock := s.lockMirrorBenchmark(userID)
		snapshot = s.loadMirrorBenchmarkSnapshot(ctx, userID)
		var fastest mirror.Entry
		var measured bool
		if !mirrorBenchmarkCoverageFresh(snapshot, ordered, freshAfter) {
			measurements := mirrorprobe.Measure(ctx, ordered, upstreamURL, s.mirrorProbe)
			snapshot = mergeMirrorBenchmarkMeasurements(snapshot, measurements, nil, time.Now().UTC())
			_ = s.saveMirrorBenchmarkSnapshot(ctx, userID, snapshot)
			fastest, measured = mirrorprobe.Fastest(measurements)
		}
		unlock()
		if measured {
			return fastest, true
		}
	}
	return fastestStableMirror(ordered, snapshot)
}

func mirrorBenchmarkCoverageFresh(snapshot mirrorBenchmarkSnapshot, entries []mirror.Entry, freshAfter time.Time) bool {
	for _, entry := range entries {
		result, ok := snapshot.Results[entry.ID]
		if !ok || result.UpdatedAt.Before(freshAfter) {
			return false
		}
	}
	return true
}

func fastestStableMirror(entries []mirror.Entry, snapshot mirrorBenchmarkSnapshot) (mirror.Entry, bool) {
	type ranked struct {
		entry     mirror.Entry
		index     int
		available bool
		score     int64
		speed     int64
		stability int
	}
	ordered := make([]ranked, 0, len(entries))
	for index, entry := range entries {
		result := snapshot.Results[entry.ID]
		latestSuccessful := len(result.Samples) > 0 && result.Samples[len(result.Samples)-1].Successful
		ordered = append(ordered, ranked{
			entry:     entry,
			index:     index,
			available: latestSuccessful && result.Status != "unavailable" && result.SpeedBytesPerSecond > 0,
			score:     result.Score,
			speed:     result.SpeedBytesPerSecond,
			stability: result.StabilityPercent,
		})
	}
	slices.SortStableFunc(ordered, func(left, right ranked) int {
		if left.available != right.available {
			if left.available {
				return -1
			}
			return 1
		}
		if left.speed != right.speed {
			if left.speed > right.speed {
				return -1
			}
			return 1
		}
		if left.stability != right.stability {
			if left.stability > right.stability {
				return -1
			}
			return 1
		}
		if left.score != right.score {
			if left.score > right.score {
				return -1
			}
			return 1
		}
		if left.index != right.index {
			return left.index - right.index
		}
		return strings.Compare(left.entry.ID, right.entry.ID)
	})
	if len(ordered) == 0 || !ordered[0].available {
		return mirror.Entry{}, false
	}
	return ordered[0].entry, true
}

func (s *Server) handleRunMirrorBenchmark(w http.ResponseWriter, r *http.Request) {
	result, err := s.runMirrorBenchmark(r.Context(), currentUserID(r))
	if err != nil {
		writeError(w, 502, "MIRROR_BENCHMARK_FAILED", "Could not evaluate GitHub mirrors")
		return
	}
	writeJSON(w, 200, map[string]any{"result": result})
}
