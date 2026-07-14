package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	entclient "lazycat.community/appstore/ent"
	apppkg "lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/user"
	"lazycat.community/appstore/internal/config"
	"lazycat.community/appstore/internal/dbpool"
)

type countingQueryer struct {
	next  queryContextExecutor
	count *atomic.Int64
}

func (q countingQueryer) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q.count.Add(1)
	return q.next.QueryContext(ctx, query, args...)
}

type appMetricsFixture struct {
	server *Server
	starts downloadPeriodStarts
	appA   int
	appB   int
}

func TestRecordAppDownloadStoresVersionSnapshot(t *testing.T) {
	testApp := newTestApp(t)
	ctx := t.Context()
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	record := testApp.server.db.App.Create().
		SetOwnerID(admin.ID).
		SetPackageID("cloud.lazycat.test.download-snapshot").
		SetName("Download Snapshot").
		SetSlug("download-snapshot").
		SetStatus(apppkg.StatusAPPROVED).
		SaveX(ctx)
	if err := testApp.server.recordAppDownload(ctx, record.ID, "3.2.0"); err != nil {
		t.Fatalf("record download: %v", err)
	}
	event := testApp.server.db.AppDownload.Query().
		Where(appdownload.AppIDEQ(record.ID)).
		OnlyX(ctx)
	if event.Version != "3.2.0" {
		t.Fatalf("version = %q, want 3.2.0", event.Version)
	}
}

func newAppMetricsFixture(t *testing.T) appMetricsFixture {
	t.Helper()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, time.July, 8, 12, 0, 0, 0, location)
	starts := downloadPeriodStartsAt(fixedNow, location)
	testApp := newTestApp(t)
	testApp.server.setNow(func() time.Time { return fixedNow })
	if err := testApp.server.setSetting(t.Context(), settingSiteTimeZone, "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	admin := testApp.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(t.Context())
	createApp := func(packageID, slug string) (int, string) {
		record := testApp.server.db.App.Create().
			SetOwnerID(admin.ID).
			SetPackageID(packageID).
			SetName(slug).
			SetSlug(slug).
			SetStatus(apppkg.StatusAPPROVED).
			SaveX(t.Context())
		version := testApp.server.db.AppVersion.Create().
			SetAppID(record.ID).
			SetUploaderID(admin.ID).
			SetVersion("1.0.0").
			SetStatus(appversion.StatusAPPROVED).
			SetSourceType(appversion.SourceTypeGITHUB).
			SetDownloadURL("https://example.com/app.lpk").
			SetPublishedAt(fixedNow).
			SaveX(t.Context())
		return record.ID, version.Version
	}
	appA, versionA := createApp("cloud.lazycat.test.metrics-a", "metrics-a")
	appB, versionB := createApp("cloud.lazycat.test.metrics-b", "metrics-b")
	for _, createdAt := range []time.Time{
		starts.Year.Add(-time.Second), starts.Year,
		starts.Month.Add(-time.Second), starts.Month,
		starts.Week.Add(-time.Second), starts.Week,
		starts.Day.Add(-time.Second), starts.Day,
	} {
		testApp.server.db.AppDownload.Create().SetAppID(appA).SetVersion(versionA).SetCreatedAt(createdAt).SaveX(t.Context())
	}
	for _, createdAt := range []time.Time{fixedNow.Add(-time.Hour), starts.Year.Add(-time.Second)} {
		testApp.server.db.AppDownload.Create().SetAppID(appB).SetVersion(versionB).SetCreatedAt(createdAt).SaveX(t.Context())
	}
	return appMetricsFixture{server: testApp.server, starts: starts, appA: appA, appB: appB}
}

func TestDownloadCountsByPeriodCandidateUsesOneQuery(t *testing.T) {
	fixture := newAppMetricsFixture(t)
	var queries atomic.Int64
	fixture.server.metricsSQL = countingQueryer{next: fixture.server.sqlDB, count: &queries}
	got, err := downloadCountsByPeriodCandidate(t.Context(), fixture.server, []int{fixture.appA, fixture.appB}, fixture.starts)
	if err != nil {
		t.Fatal(err)
	}
	assertAppMetricCounts(t, got[fixture.appA], downloadStats{Day: 1, Week: 3, Month: 5, Year: 7})
	assertAppMetricCounts(t, got[fixture.appB], downloadStats{Day: 1, Week: 1, Month: 1, Year: 1})
	if queries.Load() != 1 {
		t.Fatalf("queries = %d, want 1", queries.Load())
	}
}

func TestDownloadCountsByPeriodMatchesFourQueryBaseline(t *testing.T) {
	fixture := newAppMetricsFixture(t)
	appIDs := []int{fixture.appA, fixture.appB}
	want := map[int]downloadStats{fixture.appA: {Total: 80}, fixture.appB: {Total: 20}}
	if err := fixture.server.loadAppSummaryDownloadStats(t.Context(), appIDs, want); err != nil {
		t.Fatal(err)
	}
	got := map[int]downloadStats{fixture.appA: {Total: 80}, fixture.appB: {Total: 20}}
	periodCounts, err := downloadCountsByPeriodCandidate(t.Context(), fixture.server, appIDs, fixture.starts)
	if err != nil {
		t.Fatal(err)
	}
	for appID, counts := range periodCounts {
		stats := got[appID]
		stats.Day = counts.Day
		stats.Week = counts.Week
		stats.Month = counts.Month
		stats.Year = counts.Year
		got[appID] = stats
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("one-query stats = %#v, four-query stats = %#v", got, want)
	}
}

func TestDownloadCountsByPeriodQueryBindings(t *testing.T) {
	starts := downloadPeriodStarts{
		Day:   time.Date(2026, time.July, 8, 16, 0, 0, 0, time.UTC),
		Week:  time.Date(2026, time.July, 5, 16, 0, 0, 0, time.UTC),
		Month: time.Date(2026, time.June, 30, 16, 0, 0, 0, time.UTC),
		Year:  time.Date(2025, time.December, 31, 16, 0, 0, 0, time.UTC),
	}
	appIDs := []int{17, 23}
	for _, tc := range []struct {
		driver       string
		placeholders []string
	}{
		{driver: "sqlite3", placeholders: []string{"?", "?", "?", "?", "?,?", "?"}},
		{driver: "mysql", placeholders: []string{"?", "?", "?", "?", "?,?", "?"}},
		{driver: "postgres", placeholders: []string{"$1", "$2", "$3", "$4", "$5,$6", "$7"}},
	} {
		t.Run(tc.driver, func(t *testing.T) {
			query, args := downloadCountsByPeriodCandidateQuery(tc.driver, appIDs, starts)
			wantQuery := fmt.Sprintf(`
SELECT app_id,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS day_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS week_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS month_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS year_count
FROM app_downloads
WHERE app_id IN (%s)
  AND created_at >= %s
GROUP BY app_id`, tc.placeholders[0], tc.placeholders[1], tc.placeholders[2], tc.placeholders[3], tc.placeholders[4], tc.placeholders[5])
			if query != wantQuery {
				t.Fatalf("query = %q, want %q", query, wantQuery)
			}
			wantArgs := []any{starts.Day, starts.Week, starts.Month, starts.Year, 17, 23, starts.Year}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("args = %#v, want %#v", args, wantArgs)
			}
			if strings.Contains(query, "17") || strings.Contains(query, "23") || strings.Contains(query, starts.Year.Format(time.RFC3339)) {
				t.Fatalf("query interpolates bound data: %q", query)
			}
			if strings.Contains(query, "version") {
				t.Fatalf("query groups downloads by version: %q", query)
			}
		})
	}
}

func assertAppMetricCounts(t *testing.T, got, want downloadStats) {
	t.Helper()
	if got.Day != want.Day || got.Week != want.Week || got.Month != want.Month || got.Year != want.Year {
		t.Fatalf("download stats = %+v, want %+v", got, want)
	}
}

var benchmarkAppSummaries []appSummary

func BenchmarkPreloadAppSummaries(b *testing.B) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		b.Fatal(err)
	}
	fixedNow := time.Date(2026, time.July, 8, 12, 0, 0, 0, location)
	for _, appCount := range []int{50, 100} {
		for _, eventCount := range []int{0, 10_000, 20_000} {
			name := fmt.Sprintf("apps=%d/events=%d", appCount, eventCount)
			b.Run(name, func(b *testing.B) {
				b.Run("four_queries", func(b *testing.B) {
					benchmarkPreloadAppSummaries(b, appCount, eventCount, fixedNow, false)
				})
				b.Run("one_query", func(b *testing.B) {
					benchmarkPreloadAppSummaries(b, appCount, eventCount, fixedNow, true)
				})
			})
		}
	}
}

func benchmarkPreloadAppSummaries(b *testing.B, appCount, eventCount int, fixedNow time.Time, oneQuery bool) {
	b.Helper()
	srv, records, queries := newAppMetricsBenchmark(b, appCount, eventCount, fixedNow)
	var load func(context.Context) (appSummaryPreload, error)
	if oneQuery {
		load = func(ctx context.Context) (appSummaryPreload, error) {
			return preloadAppSummariesOneQuery(ctx, srv, records)
		}
	} else {
		load = func(ctx context.Context) (appSummaryPreload, error) {
			return srv.preloadAppSummaries(ctx, records, nil)
		}
	}
	iterations := int64(0)
	queries.Store(0)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		preload, err := load(b.Context())
		if err != nil {
			b.Fatal(err)
		}
		benchmarkAppSummaries = materializeBenchmarkSummaries(b.Context(), srv, records, preload)
		iterations++
	}
	b.StopTimer()
	b.ReportMetric(float64(queries.Load())/float64(iterations), "queries/op")
}

func newAppMetricsBenchmark(b *testing.B, appCount, eventCount int, fixedNow time.Time) (*Server, []*entclient.App, *atomic.Int64) {
	b.Helper()
	var queries atomic.Int64
	dsn := sqliteDSN(filepath.Join(b.TempDir(), "bench.db"))
	sqlDB, driver, err := dbpool.Open(dbpool.Config{
		Driver:      "sqlite3",
		DSN:         dsn,
		MaxOpen:     1,
		MaxIdle:     1,
		MaxLifetime: 30 * time.Minute,
		MaxIdleTime: 5 * time.Minute,
	})
	if err != nil {
		b.Fatal(err)
	}
	client := entclient.NewClient(
		entclient.Driver(driver),
		entclient.Debug(),
		entclient.Log(func(...any) { queries.Add(1) }),
	)
	b.Cleanup(func() {
		if err := client.Close(); err != nil {
			b.Errorf("close benchmark client: %v", err)
		}
	})
	if err := client.Schema.Create(b.Context()); err != nil {
		b.Fatal(err)
	}
	client.SiteSetting.Create().SetKey(settingSiteTimeZone).SetValue("Asia/Shanghai").SaveX(b.Context())
	srv := &Server{
		cfg:        config.Config{DBDriver: "sqlite3"},
		db:         client,
		sqlDB:      sqlDB,
		metricsSQL: countingQueryer{next: sqlDB, count: &queries},
		now:        func() time.Time { return fixedNow },
	}
	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SetRole(user.RoleUSER).
		SaveX(b.Context())
	category := client.Category.Create().SetName("Tools").SetSlug("tools").SaveX(b.Context())
	tagA := client.Tag.Create().SetName("Productivity").SetSlug("productivity").SaveX(b.Context())
	tagB := client.Tag.Create().SetName("Utility").SetSlug("utility").SaveX(b.Context())
	downloadCounts := make([]int, appCount)
	for i := range eventCount {
		downloadCounts[i%appCount]++
	}
	records := make([]*entclient.App, 0, appCount)
	versions := make([]string, 0, appCount)
	for i := range appCount {
		record := client.App.Create().
			SetOwnerID(owner.ID).
			SetCategoryID(category.ID).
			SetPackageID(fmt.Sprintf("cloud.lazycat.bench.%03d", i)).
			SetName(fmt.Sprintf("Bench App %03d", i)).
			SetSlug(fmt.Sprintf("bench-app-%03d", i)).
			SetSummary("Benchmark app").
			SetDownloadCount(downloadCounts[i]).
			SetStatus(apppkg.StatusAPPROVED).
			SaveX(b.Context())
		client.AppTag.Create().SetAppID(record.ID).SetTagID(tagA.ID).SaveX(b.Context())
		client.AppTag.Create().SetAppID(record.ID).SetTagID(tagB.ID).SaveX(b.Context())
		version := client.AppVersion.Create().
			SetAppID(record.ID).
			SetUploaderID(owner.ID).
			SetVersion("1.0.0").
			SetStatus(appversion.StatusAPPROVED).
			SetPublishedAt(fixedNow).
			SaveX(b.Context())
		records = append(records, record)
		versions = append(versions, version.Version)
	}
	seedAppMetricBenchmarkEvents(b, client, records, versions, eventCount, downloadPeriodStartsAt(fixedNow, fixedNow.Location()))
	queries.Store(0)
	return srv, records, &queries
}

func seedAppMetricBenchmarkEvents(
	b *testing.B,
	client *entclient.Client,
	records []*entclient.App,
	versions []string,
	eventCount int,
	starts downloadPeriodStarts,
) {
	b.Helper()
	if eventCount == 0 {
		return
	}
	buckets := [...]time.Time{
		starts.Day.Add(time.Hour),
		starts.Day.Add(-time.Hour),
		starts.Week.Add(-time.Hour),
		starts.Month.Add(-time.Hour),
	}
	const batchSize = 200
	for offset := 0; offset < eventCount; offset += batchSize {
		end := min(offset+batchSize, eventCount)
		builders := make([]*entclient.AppDownloadCreate, 0, end-offset)
		for eventIndex := offset; eventIndex < end; eventIndex++ {
			appIndex := eventIndex % len(records)
			round := eventIndex / len(records)
			createdAt := buckets[round%len(buckets)].Add(time.Duration(round/len(buckets)) * time.Microsecond)
			builders = append(builders, client.AppDownload.Create().
				SetAppID(records[appIndex].ID).
				SetVersion(versions[appIndex]).
				SetCreatedAt(createdAt))
		}
		client.AppDownload.CreateBulk(builders...).SaveX(b.Context())
	}
}

func preloadAppSummariesOneQuery(ctx context.Context, s *Server, apps []*entclient.App) (appSummaryPreload, error) {
	data := appSummaryPreload{
		owners:             map[int]string{},
		categories:         map[int]*entclient.Category{},
		tags:               map[int][]string{},
		visibleGroupIDs:    map[int][]int{},
		latestVersions:     map[int]*entclient.AppVersion{},
		collaboratorAppIDs: map[int]struct{}{},
		appFavorites:       map[int]bool{},
		submitterFavorites: map[int]bool{},
		downloadStats:      map[int]downloadStats{},
		ratings:            map[int]ratingSummary{},
	}
	if len(apps) == 0 {
		return data, nil
	}
	data.commentsEnabled = s.commentsEnabled(ctx)
	appIDs := make([]int, 0, len(apps))
	ownerIDs := map[int]struct{}{}
	categoryIDs := map[int]struct{}{}
	for _, record := range apps {
		appIDs = append(appIDs, record.ID)
		ownerIDs[record.OwnerID] = struct{}{}
		if record.CategoryID != nil {
			categoryIDs[*record.CategoryID] = struct{}{}
		}
	}
	if err := s.loadAppSummaryOwners(ctx, ownerIDs, data.owners); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryCategories(ctx, categoryIDs, data.categories); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryTags(ctx, appIDs, data.tags); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryVisibleGroups(ctx, appIDs, data.visibleGroupIDs); err != nil {
		return data, err
	}
	if err := s.loadAppSummaryLatestVersions(ctx, appIDs, data.latestVersions); err != nil {
		return data, err
	}
	starts := downloadPeriodStartsAt(s.nowUTC(), s.siteLocation(ctx))
	periodCounts, err := downloadCountsByPeriodCandidate(ctx, s, appIDs, starts)
	if err != nil {
		return data, err
	}
	for appID, counts := range periodCounts {
		data.downloadStats[appID] = counts
	}
	if err := s.loadAppSummaryRatings(ctx, appIDs, nil, data.ratings); err != nil {
		return data, err
	}
	return data, nil
}

func downloadCountsByPeriodCandidate(
	ctx context.Context,
	s *Server,
	appIDs []int,
	starts downloadPeriodStarts,
) (map[int]downloadStats, error) {
	if len(appIDs) == 0 {
		return map[int]downloadStats{}, nil
	}
	queryer := s.metricsSQL
	if queryer == nil {
		queryer = s.sqlDB
	}
	if queryer == nil {
		return nil, errors.New("app metrics database is not configured")
	}
	query, args := downloadCountsByPeriodCandidateQuery(s.cfg.DBDriver, appIDs, starts)
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int]downloadStats, len(appIDs))
	for rows.Next() {
		var appID int
		var stats downloadStats
		if err := rows.Scan(&appID, &stats.Day, &stats.Week, &stats.Month, &stats.Year); err != nil {
			return nil, err
		}
		out[appID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func downloadCountsByPeriodCandidateQuery(driver string, appIDs []int, starts downloadPeriodStarts) (string, []any) {
	query := fmt.Sprintf(`
SELECT app_id,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS day_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS week_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS month_count,
       SUM(CASE WHEN created_at >= %s THEN 1 ELSE 0 END) AS year_count
FROM app_downloads
WHERE app_id IN (%s)
  AND created_at >= %s
GROUP BY app_id`,
		appMetricBindVars(driver, 1, 1),
		appMetricBindVars(driver, 2, 1),
		appMetricBindVars(driver, 3, 1),
		appMetricBindVars(driver, 4, 1),
		appMetricBindVars(driver, 5, len(appIDs)),
		appMetricBindVars(driver, 5+len(appIDs), 1),
	)
	args := make([]any, 0, 5+len(appIDs))
	args = append(args, starts.Day, starts.Week, starts.Month, starts.Year)
	for _, appID := range appIDs {
		args = append(args, appID)
	}
	args = append(args, starts.Year)
	return query, args
}

func appMetricBindVars(driver string, start, count int) string {
	vars := make([]string, count)
	for i := range count {
		if driver == "postgres" {
			vars[i] = "$" + strconv.Itoa(start+i)
		} else {
			vars[i] = "?"
		}
	}
	return strings.Join(vars, ",")
}

func materializeBenchmarkSummaries(ctx context.Context, srv *Server, records []*entclient.App, preload appSummaryPreload) []appSummary {
	out := make([]appSummary, 0, len(records))
	for _, record := range records {
		out = append(out, srv.appSummaryDTOFromPreload(ctx, record, nil, preload))
	}
	return out
}
