package clientserver

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientinstallhistory"
)

func TestHistoryRetentionByCountAndAgeIsUserScoped(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		remaining      int
	}{
		{"count", `{"historyMaxEntries":2,"historyRetentionDays":0}`, 2},
		{"age", `{"historyMaxEntries":0,"historyRetentionDays":7}`, 3},
		{"both", `{"historyMaxEntries":2,"historyRetentionDays":7}`, 2},
		{"unlimited", `{"historyMaxEntries":0,"historyRetentionDays":0}`, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testServer(t)
			ctx := t.Context()
			for _, userID := range []string{"alice", "bob"} {
				for i := range 4 {
					app.server.db.ClientInstallHistory.Create().SetUserID(userID).SetPackageID("test.app").SetAppName("Test").SetCreatedAt(time.Now().Add(-time.Duration(i*3) * 24 * time.Hour)).SaveX(ctx)
				}
			}
			response := app.request(http.MethodPatch, "/api/client/v1/settings", tc.settings, "alice")
			if response.Code != http.StatusOK {
				t.Fatalf("settings: %d %s", response.Code, response.Body.String())
			}
			response = app.request(http.MethodPost, "/api/client/v1/history/prune", "", "alice")
			if response.Code != http.StatusOK {
				t.Fatalf("prune: %d %s", response.Code, response.Body.String())
			}
			if got := app.server.db.ClientInstallHistory.Query().Where(clientinstallhistory.UserIDEQ("alice")).CountX(ctx); got != tc.remaining {
				t.Fatalf("alice count = %d, want %d", got, tc.remaining)
			}
			if got := app.server.db.ClientInstallHistory.Query().Where(clientinstallhistory.UserIDEQ("bob")).CountX(ctx); got != 4 {
				t.Fatalf("bob count = %d", got)
			}
		})
	}
}

func TestHistoryRetentionDefaultsOnReadAndWrite(t *testing.T) {
	app := testServer(t)
	ctx := t.Context()
	now := time.Now()
	app.server.db.ClientInstallHistory.Create().SetUserID("alice").SetPackageID("test.app").SetAppName("Old").SetCreatedAt(now.AddDate(0, 0, -91)).SaveX(ctx)
	var newestID int
	for range 1003 {
		row := app.server.db.ClientInstallHistory.Create().SetUserID("alice").SetPackageID("test.app").SetAppName("Test").SetCreatedAt(now).SaveX(ctx)
		newestID = row.ID
	}
	response := app.request(http.MethodGet, "/api/client/v1/history", "", "alice")
	if response.Code != http.StatusOK {
		t.Fatalf("read: %d %s", response.Code, response.Body.String())
	}
	rows := app.server.db.ClientInstallHistory.Query().Where(clientinstallhistory.UserIDEQ("alice")).Order(ent.Desc(clientinstallhistory.FieldID)).AllX(ctx)
	if len(rows) != 500 || rows[0].ID != newestID || rows[len(rows)-1].ID != newestID-499 {
		t.Fatalf("default count/tie order incorrect: count=%d", len(rows))
	}
	if err := app.server.recordInstallHistory(ctx, "alice", &ent.ClientSourceApp{ID: 1, SourceID: 1}, SourceAppDTO{Name: "Test", PackageID: "test.app"}, nil, clientinstallhistory.ResultFAILED, "failed"); err != nil {
		t.Fatal(err)
	}
	if got := app.server.db.ClientInstallHistory.Query().CountX(ctx); got != 500 {
		t.Fatalf("record retained %d", got)
	}
}

func TestHistoryRetentionInvalidAndOmittedSettings(t *testing.T) {
	app := testServer(t)
	for _, body := range []string{`{"historyMaxEntries":-1}`, `{"historyMaxEntries":10001}`, `{"historyRetentionDays":3651}`} {
		if got := app.request(http.MethodPatch, "/api/client/v1/settings", body, "alice"); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid limit accepted: %s", got.Body.String())
		}
	}
	response := app.request(http.MethodPatch, "/api/client/v1/settings", `{"historyMaxEntries":100,"historyRetentionDays":7}`, "alice")
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	response = app.request(http.MethodPatch, "/api/client/v1/settings", `{}`, "alice")
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	settings, err := app.server.clientSettings(t.Context(), "alice")
	if err != nil || settings.HistoryMaxEntries != 100 || settings.HistoryRetentionDays != 7 {
		t.Fatalf("settings lost: %#v %v", settings, err)
	}
}

func TestHistoryRetentionReadFailureDoesNotDelete(t *testing.T) {
	app := testServer(t)
	ctx := t.Context()
	app.server.db.ClientInstallHistory.Create().SetUserID("alice").SetPackageID("test.app").SetAppName("Old").SetCreatedAt(time.Now().AddDate(0, 0, -100)).SaveX(ctx)
	app.server.db.ClientSetting.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
			return nil, errors.New("injected read failure")
		})
	}))
	if _, err := app.server.pruneInstallHistory(ctx, "alice"); err == nil {
		t.Fatal("expected read error")
	}
	if got := app.server.db.ClientInstallHistory.Query().CountX(ctx); got != 1 {
		t.Fatal("record deleted after settings read failure")
	}
}
