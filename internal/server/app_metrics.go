package server

import (
	"context"
	"database/sql"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appdownload"
	"lazycat.community/appstore/ent/appvote"
)

type queryContextExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Server) nowUTC() time.Time {
	if s != nil {
		return s.currentTime().UTC()
	}
	return time.Now().UTC()
}

type downloadPeriod string

const (
	downloadPeriodDay   downloadPeriod = "day"
	downloadPeriodWeek  downloadPeriod = "week"
	downloadPeriodMonth downloadPeriod = "month"
	downloadPeriodYear  downloadPeriod = "year"
)

func downloadPeriodFromSort(value string) (downloadPeriod, bool) {
	switch value {
	case "downloads_day":
		return downloadPeriodDay, true
	case "downloads_week":
		return downloadPeriodWeek, true
	case "downloads_month":
		return downloadPeriodMonth, true
	case "downloads_year":
		return downloadPeriodYear, true
	default:
		return "", false
	}
}

func (s *Server) applyAppListSort(ctx context.Context, q *entgo.AppQuery, sort string) {
	if period, ok := downloadPeriodFromSort(sort); ok {
		start := downloadPeriodStart(s.nowUTC(), s.siteLocation(ctx), period).UTC()
		q.Order(orderAppsByDownloadPeriod(start), entgo.Desc(app.FieldUpdatedAt))
		return
	}
	switch sort {
	case "downloads":
		q.Order(entgo.Desc(app.FieldDownloadCount), entgo.Desc(app.FieldUpdatedAt))
	case "name":
		q.Order(entgo.Asc(app.FieldName), entgo.Desc(app.FieldUpdatedAt))
	default:
		q.Order(entgo.Desc(app.FieldUpdatedAt))
	}
}

type downloadPeriodStarts struct {
	Day   time.Time
	Week  time.Time
	Month time.Time
	Year  time.Time
}

func downloadPeriodStartsAt(now time.Time, loc *time.Location) downloadPeriodStarts {
	return downloadPeriodStarts{
		Day:   downloadPeriodStart(now, loc, downloadPeriodDay).UTC(),
		Week:  downloadPeriodStart(now, loc, downloadPeriodWeek).UTC(),
		Month: downloadPeriodStart(now, loc, downloadPeriodMonth).UTC(),
		Year:  downloadPeriodStart(now, loc, downloadPeriodYear).UTC(),
	}
}

func orderAppsByDownloadPeriod(start time.Time) app.OrderOption {
	return func(selector *entsql.Selector) {
		downloads := entsql.Table(appdownload.Table)
		counts := entsql.Select().
			From(downloads).
			Where(entsql.And(
				entsql.ColumnsEQ(downloads.C(appdownload.FieldAppID), selector.C(app.FieldID)),
				entsql.GTE(downloads.C(appdownload.FieldCreatedAt), start),
			)).
			Count()
		selector.OrderExpr(entsql.ExprFunc(func(b *entsql.Builder) {
			b.Wrap(func(b *entsql.Builder) {
				b.Join(counts)
			})
			b.WriteString(" DESC")
		}))
	}
}

func downloadPeriodStart(now time.Time, loc *time.Location, period downloadPeriod) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	y, m, d := local.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
	switch period {
	case downloadPeriodDay:
		return dayStart
	case downloadPeriodWeek:
		dayOffset := (int(local.Weekday()) + 6) % 7
		return dayStart.AddDate(0, 0, -dayOffset)
	case downloadPeriodMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)
	case downloadPeriodYear:
		return time.Date(y, time.January, 1, 0, 0, 0, 0, loc)
	default:
		return dayStart
	}
}

func (s *Server) loadAppSummaryDownloadStats(ctx context.Context, appIDs []int, out map[int]downloadStats) error {
	if len(appIDs) == 0 {
		return nil
	}
	starts := downloadPeriodStartsAt(s.nowUTC(), s.siteLocation(ctx))
	dayCounts, err := s.downloadCountsSince(ctx, appIDs, starts.Day)
	if err != nil {
		return err
	}
	weekCounts, err := s.downloadCountsSince(ctx, appIDs, starts.Week)
	if err != nil {
		return err
	}
	monthCounts, err := s.downloadCountsSince(ctx, appIDs, starts.Month)
	if err != nil {
		return err
	}
	yearCounts, err := s.downloadCountsSince(ctx, appIDs, starts.Year)
	if err != nil {
		return err
	}
	for _, appID := range appIDs {
		stats := out[appID]
		stats.Day = dayCounts[appID]
		stats.Week = weekCounts[appID]
		stats.Month = monthCounts[appID]
		stats.Year = yearCounts[appID]
		out[appID] = stats
	}
	return nil
}

func (s *Server) downloadStatsForApp(ctx context.Context, appID, total int) downloadStats {
	out := map[int]downloadStats{appID: {Total: total}}
	if err := s.loadAppSummaryDownloadStats(ctx, []int{appID}, out); err != nil {
		return downloadStats{Total: total}
	}
	stats := out[appID]
	stats.Total = total
	return stats
}

func (s *Server) downloadCountsSince(ctx context.Context, appIDs []int, start time.Time) (map[int]int, error) {
	type countRow struct {
		AppID int `json:"app_id"`
		Count int `json:"count"`
	}
	var rows []countRow
	if err := s.db.AppDownload.Query().
		Where(appdownload.AppIDIn(appIDs...), appdownload.CreatedAtGTE(start)).
		GroupBy(appdownload.FieldAppID).
		Aggregate(entgo.As(entgo.Count(), "count")).
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[int]int, len(rows))
	for _, row := range rows {
		out[row.AppID] = row.Count
	}
	return out, nil
}

func (s *Server) loadAppSummaryRatings(ctx context.Context, appIDs []int, u *entgo.User, out map[int]ratingSummary) error {
	if len(appIDs) == 0 {
		return nil
	}
	type countRow struct {
		AppID int `json:"app_id"`
		Count int `json:"count"`
	}
	var rows []countRow
	if err := s.db.AppVote.Query().
		Where(appvote.AppIDIn(appIDs...)).
		GroupBy(appvote.FieldAppID).
		Aggregate(entgo.As(entgo.Count(), "count")).
		Scan(ctx, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		out[row.AppID] = ratingSummary{
			Score:     ratingScore(row.Count),
			VoteCount: row.Count,
		}
	}
	if u == nil {
		return nil
	}
	votes, err := s.db.AppVote.Query().
		Where(appvote.AppIDIn(appIDs...), appvote.UserIDEQ(u.ID)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, vote := range votes {
		rating := out[vote.AppID]
		rating.Voted = true
		out[vote.AppID] = rating
	}
	return nil
}

func (s *Server) ratingForApp(ctx context.Context, appID int, u *entgo.User) ratingSummary {
	out := map[int]ratingSummary{}
	if err := s.loadAppSummaryRatings(ctx, []int{appID}, u, out); err != nil {
		return ratingSummary{}
	}
	return out[appID]
}

func ratingScore(voteCount int) float64 {
	return float64(voteCount)
}

func (s *Server) recordAppDownload(ctx context.Context, appID int, version string) error {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.App.UpdateOneID(appID).AddDownloadCount(1).Save(ctx); err != nil {
		return err
	}
	if _, err := tx.AppDownload.Create().
		SetAppID(appID).
		SetVersion(strings.TrimSpace(version)).
		SetCreatedAt(s.nowUTC()).
		Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}
