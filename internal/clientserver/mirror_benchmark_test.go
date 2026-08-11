package clientserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/mirrorprobe"
)

func TestMirrorBenchmarkPrefersFastestCurrentlyAvailableMirror(t *testing.T) {
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	snapshot := mirrorBenchmarkSnapshot{Version: 1, Results: map[string]mirrorBenchmarkResult{}}
	measurements := []mirrorprobe.Measurement{
		{Entry: mirror.Entry{ID: "fast", Kind: mirror.KindDownload}, Successful: true, BytesPerSecond: 12_000_000},
		{Entry: mirror.Entry{ID: "stable", Kind: mirror.KindDownload}, Successful: true, BytesPerSecond: 8_000_000},
	}
	for index := range 4 {
		snapshot = mergeMirrorBenchmarkMeasurements(snapshot, measurements, nil, now.Add(time.Duration(index)*time.Hour))
	}
	measurements[0].Successful = false
	measurements[0].BytesPerSecond = 0
	for index := range 3 {
		snapshot = mergeMirrorBenchmarkMeasurements(snapshot, measurements[:1], nil, now.Add(time.Duration(index+4)*time.Hour))
	}

	entries := []mirror.Entry{{ID: "fast", Kind: mirror.KindDownload}, {ID: "stable", Kind: mirror.KindDownload}}
	preferred, ok := fastestStableMirror(entries, snapshot)
	if !ok || preferred.ID != "stable" {
		t.Fatalf("preferred = %#v, %t; results = %#v", preferred, ok, snapshot.Results)
	}
	if snapshot.Results["fast"].Status != "unavailable" {
		t.Fatalf("fast status = %q", snapshot.Results["fast"].Status)
	}

	snapshot = mergeMirrorBenchmarkMeasurements(snapshot, []mirrorprobe.Measurement{
		{Entry: mirror.Entry{ID: "fast", Kind: mirror.KindDownload}, Successful: true, BytesPerSecond: 12_000_000},
	}, nil, now.Add(8*time.Hour))
	preferred, ok = fastestStableMirror(entries, snapshot)
	if !ok || preferred.ID != "fast" {
		t.Fatalf("fastest available mirror was not preferred: %#v, %t; results = %#v", preferred, ok, snapshot.Results)
	}
	if snapshot.Results["fast"].Status != "unstable" || snapshot.Results["fast"].Score >= snapshot.Results["stable"].Score {
		t.Fatalf("test must prove speed wins even when the stability score is lower: %#v", snapshot.Results)
	}
}

func TestMirrorBenchmarkDueUsesConfiguredInterval(t *testing.T) {
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	recent := now.Add(-59 * time.Minute)
	old := now.Add(-61 * time.Minute)
	if mirrorBenchmarkDue(false, 60, &old, now) {
		t.Fatal("disabled benchmark should not be due")
	}
	if mirrorBenchmarkDue(true, 60, &recent, now) {
		t.Fatal("recent benchmark should not be due")
	}
	if !mirrorBenchmarkDue(true, 60, &old, now) || !mirrorBenchmarkDue(true, 60, nil, now) {
		t.Fatal("old or missing benchmark should be due")
	}
}

func TestMirrorBenchmarkSnapshotIsBounded(t *testing.T) {
	snapshot := mirrorBenchmarkSnapshot{Version: 1, Results: map[string]mirrorBenchmarkResult{}}
	now := time.Now().UTC()
	for index := range 300 {
		id := fmt.Sprintf("mirror-%03d", index)
		snapshot = mergeMirrorBenchmarkMeasurements(snapshot, []mirrorprobe.Measurement{{
			Entry:          mirror.Entry{ID: id, Kind: mirror.KindDownload},
			BytesPerSecond: int64(index + 1),
			Successful:     true,
		}}, nil, now.Add(time.Duration(index)*time.Second))
	}
	if got := len(snapshot.Results); got > 256 {
		t.Fatalf("snapshot retained %d mirror results, want at most 256", got)
	}
}

func TestNormalizeFeedMirrorsRejectsUnboundedProbeTargets(t *testing.T) {
	entries := make([]mirror.Entry, maxSourceFeedMirrors+1)
	for index := range entries {
		entries[index] = mirror.Entry{
			Kind: mirror.KindDownload,
			Name: "Mirror " + strconv.Itoa(index),
			URL:  "https://mirror-" + strconv.Itoa(index) + ".example/https://github.com",
		}
	}
	if _, err := normalizeFeedMirrors(entries); err == nil || !strings.Contains(err.Error(), "more than 16") {
		t.Fatalf("normalizeFeedMirrors() error = %v", err)
	}
}

func TestFastestStableMirrorRejectsUnavailableResults(t *testing.T) {
	entries := []mirror.Entry{{ID: "failed", Kind: mirror.KindDownload}}
	snapshot := mirrorBenchmarkSnapshot{Version: 1, Results: map[string]mirrorBenchmarkResult{
		"failed": {
			ID:     "failed",
			Kind:   mirror.KindDownload,
			Status: "unavailable",
			Samples: []mirrorBenchmarkSample{{
				At:         time.Now(),
				Successful: false,
			}},
		},
	}}
	if selected, ok := fastestStableMirror(entries, snapshot); ok {
		t.Fatalf("unavailable mirror selected: %#v", selected)
	}
}

func TestRunMirrorBenchmarkPersistsDeviceLocalResults(t *testing.T) {
	app := testServer(t)
	firstURL := "https://first.example/https://github.com"
	secondURL := "https://second.example/https://github.com"
	mirrorsJSON, err := json.Marshal([]mirror.Entry{
		{ID: mirror.ID(mirror.KindDownload, firstURL), Kind: mirror.KindDownload, Name: "First", URL: firstURL},
		{ID: mirror.ID(mirror.KindDownload, secondURL), Kind: mirror.KindDownload, Name: "Second", URL: secondURL},
	})
	if err != nil {
		t.Fatal(err)
	}
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Feed").
		SetURL("https://feed.example/source.json").
		SetMirrorsJSON(string(mirrorsJSON)).
		SaveX(t.Context())
	latest, err := json.Marshal(VersionDTO{
		Version:             "1.0.0",
		DownloadURL:         "https://github.com/acme/app/releases/download/v1/app.lpk",
		UpstreamDownloadURL: "https://github.com/acme/app/releases/download/v1/app.lpk",
	})
	if err != nil {
		t.Fatal(err)
	}
	app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetPackageID("community.lazycat.benchmark").
		SetName("Benchmark").
		SetSlug("benchmark").
		SetLatestVersionJSON(string(latest)).
		SaveX(t.Context())
	app.server.mirrorProbe = func(_ context.Context, rawURL string) (int64, error) {
		if strings.Contains(rawURL, "second.example") {
			return 15_000_000, nil
		}
		return 4_000_000, nil
	}

	result, err := app.server.runMirrorBenchmark(t.Context(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if result.Tested != 2 || result.Available != 2 {
		t.Fatalf("result = %#v", result)
	}
	snapshot := app.server.loadMirrorBenchmarkSnapshot(t.Context(), "alice")
	preferred, ok := fastestStableMirror(sourceMirrors(source), snapshot)
	if !ok || preferred.URL != secondURL {
		t.Fatalf("preferred = %#v, %t", preferred, ok)
	}
	if snapshot.Results[mirror.ID(mirror.KindDownload, secondURL)].StabilityPercent != 100 {
		t.Fatalf("snapshot = %#v", snapshot)
	}

	if err := app.server.setClientSettingValue(t.Context(), "alice", settingMirrorBenchmarkEnabled, "true"); err != nil {
		t.Fatal(err)
	}
	if err := app.server.setClientSettingValue(t.Context(), "alice", settingMirrorBenchmarkIntervalMinutes, "30"); err != nil {
		t.Fatal(err)
	}
	if err := app.server.setClientSettingValue(t.Context(), "alice", settingMirrorBenchmarkLastRunAt, time.Now().Add(-31*time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	var scheduledProbes atomic.Int32
	app.server.mirrorProbe = func(_ context.Context, rawURL string) (int64, error) {
		scheduledProbes.Add(1)
		if strings.Contains(rawURL, "second.example") {
			return 15_000_000, nil
		}
		return 4_000_000, nil
	}
	scheduler := &sourceSyncScheduler{server: app.server, running: make(map[string]struct{})}
	if err := scheduler.runDueMirrorBenchmarks(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if got := scheduledProbes.Load(); got != 2 {
		t.Fatalf("scheduled probes = %d, want 2", got)
	}
	if err := scheduler.runDueMirrorBenchmarks(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if got := scheduledProbes.Load(); got != 2 {
		t.Fatalf("not-due scheduler repeated probes: %d", got)
	}

	rec := app.request("POST", "/api/client/v1/mirrors/benchmark", ``, "alice")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tested":2`) || !strings.Contains(rec.Body.String(), `"available":2`) {
		t.Fatalf("manual benchmark = %d %s", rec.Code, rec.Body.String())
	}
	if got := scheduledProbes.Load(); got != 4 {
		t.Fatalf("manual benchmark probes = %d, want 4 total", got)
	}
}

func TestAutoSyncScanDoesNotRunMirrorBenchmarks(t *testing.T) {
	app := testServer(t)
	mirrorURL := "https://mirror.example/https://github.com"
	mirrorsJSON, err := json.Marshal([]mirror.Entry{{
		ID: mirror.ID(mirror.KindDownload, mirrorURL), Kind: mirror.KindDownload, Name: "Mirror", URL: mirrorURL,
	}})
	if err != nil {
		t.Fatal(err)
	}
	source := app.server.db.ClientSource.Create().
		SetUserID("alice").
		SetName("Feed").
		SetURL("https://feed.example/source.json").
		SetMirrorsJSON(string(mirrorsJSON)).
		SaveX(t.Context())
	latest, err := json.Marshal(VersionDTO{
		Version:             "1.0.0",
		DownloadURL:         "https://github.com/acme/app/releases/download/v1/app.lpk",
		UpstreamDownloadURL: "https://github.com/acme/app/releases/download/v1/app.lpk",
	})
	if err != nil {
		t.Fatal(err)
	}
	app.server.db.ClientSourceApp.Create().
		SetSourceID(source.ID).
		SetPackageID("community.lazycat.scheduler-benchmark").
		SetName("Scheduler Benchmark").
		SetSlug("scheduler-benchmark").
		SetLatestVersionJSON(string(latest)).
		SaveX(t.Context())
	if err := app.server.setClientSettingValue(t.Context(), "alice", settingMirrorBenchmarkEnabled, "true"); err != nil {
		t.Fatal(err)
	}

	var probes atomic.Int32
	app.server.mirrorProbe = func(context.Context, string) (int64, error) {
		probes.Add(1)
		return 1_000_000, nil
	}
	scheduler := &sourceSyncScheduler{server: app.server, running: make(map[string]struct{})}
	if err := scheduler.runDueAutoSyncs(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if got := probes.Load(); got != 0 {
		t.Fatalf("auto-sync scan ran %d mirror probes, want an independent benchmark job", got)
	}
}
