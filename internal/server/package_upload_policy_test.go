package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/appversion"
	"lazycat.community/appstore/ent/user"
)

func TestPackageUploadSettingAndPublicCapability(t *testing.T) {
	store := newTestApp(t)
	admin := store.server.db.User.Query().Where(user.UsernameEQ("admin")).OnlyX(t.Context())
	store.cookies = []*http.Cookie{store.serverCookieFor(admin.ID)}
	assertPolicy := func(want bool) {
		t.Helper()
		response := store.do(http.MethodGet, "/api/v1/admin/settings", nil)
		var settings struct {
			Settings map[string]string `json:"settings"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &settings) != nil {
			t.Fatalf("settings status = %d, body = %s", response.Code, response.Body.String())
		}
		if got := settings.Settings[settingAllowPackageUpload]; got != strconv.FormatBool(want) {
			t.Fatalf("allow_package_upload = %q, want %t", got, want)
		}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/site/profile", nil)
		response = httptest.NewRecorder()
		store.handler.ServeHTTP(response, request)
		var profile struct {
			Site struct {
				PackageUpload struct {
					Allowed *bool `json:"allowed"`
				} `json:"packageUpload"`
			} `json:"site"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &profile) != nil {
			t.Fatalf("public profile status = %d, body = %s", response.Code, response.Body.String())
		}
		if got := profile.Site.PackageUpload.Allowed; got == nil || *got != want {
			t.Fatalf("public package upload capability missing or incorrect: %s", response.Body.String())
		}
	}

	assertPolicy(true)
	response := store.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{settingAllowPackageUpload: "invalid"})
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "must be a boolean") {
		t.Fatalf("invalid policy status = %d, body = %s", response.Code, response.Body.String())
	}
	assertPolicy(true)
	for _, allowed := range []bool{false, true} {
		response = store.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{settingAllowPackageUpload: strconv.FormatBool(allowed)})
		if response.Code != http.StatusOK {
			t.Fatalf("update policy status = %d, body = %s", response.Code, response.Body.String())
		}
		assertPolicy(allowed)
	}
	member := store.server.db.User.Create().SetUsername("policy-member").SetPasswordHash("unused").SaveX(t.Context())
	store.cookies = []*http.Cookie{store.serverCookieFor(member.ID)}
	response = store.do(http.MethodPatch, "/api/v1/admin/settings", map[string]string{settingAllowPackageUpload: "false"})
	if response.Code != http.StatusForbidden {
		t.Fatalf("member changed policy: status = %d, body = %s", response.Code, response.Body.String())
	}
	if !store.server.packageUploadAllowed(t.Context()) {
		t.Fatal("rejected policy change affected the setting")
	}
}

func TestPackageUploadPolicyForCookieAndAPIToken(t *testing.T) {
	for _, role := range []user.Role{user.RoleUSER, user.RoleSOFTWARE_ADMIN, user.RoleSITE_ADMIN} {
		for _, authMode := range []string{"cookie", "api-token"} {
			t.Run(string(role)+"/"+authMode, func(t *testing.T) {
				store := newTestApp(t)
				ctx := t.Context()
				publisher := store.server.db.User.Create().SetUsername("publisher").SetPasswordHash("unused").SetEmailVerified(true).SetRole(role).SaveX(ctx)
				if authMode == "cookie" {
					store.cookies = []*http.Cookie{store.serverCookieFor(publisher.ID)}
				} else {
					token := "lcst_package_upload_policy_token"
					store.server.db.APIToken.Create().SetUserID(publisher.ID).SetName("Publisher").SetPrefix(tokenPrefix(token)).SetTokenHash(tokenHash(token)).SaveX(ctx)
					handler := store.handler
					store.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						r.Header.Set("Authorization", "Bearer "+token)
						handler.ServeHTTP(w, r)
					})
				}
				if err := store.server.setSetting(ctx, settingAutomaticLPKInspectionWaitSeconds, "0"); err != nil {
					t.Fatal(err)
				}

				packageID := "cloud.lazycat.test.package-upload-policy"
				lpk := testLPKArchive(t, packageID, "1.0.0", "Upload Policy", "Upload policy test")
				response := store.doMultipart(http.MethodPost, "/api/v1/apps", "file", "app.lpk", lpk, nil)
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				record := store.server.db.App.Query().Where(app.PackageIDEQ(packageID)).OnlyX(ctx)
				versionPath := "/api/v1/apps/" + strconv.Itoa(record.ID) + "/versions"
				response = store.doMultipart(http.MethodPost, versionPath, "file", "app.lpk", lpk, map[string]string{"version": "1.0.1"})
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				localVersion := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(record.ID), appversion.VersionEQ("1.0.0")).OnlyX(ctx)

				if err := store.server.setSetting(ctx, settingAllowPackageUpload, "false"); err != nil {
					t.Fatal(err)
				}
				before := packagePolicyRecordCounts(t, store)
				for _, deniedRequest := range []struct {
					path        string
					contentType string
				}{
					{"/api/v1/apps", "multipart/form-data; boundary=policy"},
					{versionPath, "multipart/form-data; boundary=policy"},
					{"/api/v1/apps", "MULTIPART/FORM-DATA; boundary=policy"},
					{versionPath, "MULTIPART/FORM-DATA; boundary=policy"},
					{"/api/v1/apps", "multipart/form-data"},
					{versionPath, "multipart/form-data"},
				} {
					body := &packagePolicyUnreadBody{}
					request := httptest.NewRequest(http.MethodPost, deniedRequest.path, body)
					request.Header.Set("Content-Type", deniedRequest.contentType)
					request.ContentLength = store.server.cfg.MaxLPKSize * 100
					for _, cookie := range store.cookies {
						request.AddCookie(cookie)
					}
					response = httptest.NewRecorder()
					store.handler.ServeHTTP(response, request)
					assertPackagePolicyDenied(t, response)
					if body.reads != 0 {
						t.Fatalf("disabled upload read request body %d times", body.reads)
					}
				}
				for _, downloadURL := range []string{"", " \t "} {
					response = store.do(http.MethodPost, "/api/v1/apps", map[string]any{
						"name": "Metadata only", "packageId": "cloud.lazycat.test.metadata-only", "downloadUrl": downloadURL,
					})
					assertPackagePolicyDenied(t, response)
				}
				if after := packagePolicyRecordCounts(t, store); after != before {
					t.Fatalf("rejected requests mutated records: before = %v, after = %v", before, after)
				}

				response = store.do(http.MethodPost, "/api/v1/apps", map[string]any{
					"packageId": "cloud.lazycat.test.url-only", "name": "URL Only", "version": "2.0.0",
					"downloadUrl": "https://github.com/acme/app/releases/download/v2/app.lpk", "sha256": strings.Repeat("a", 64),
				})
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				externalApp := store.server.db.App.Query().Where(app.PackageIDEQ("cloud.lazycat.test.url-only")).OnlyX(ctx)
				response = store.do(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(externalApp.ID)+"/versions", map[string]any{
					"version": "2.0.1", "downloadUrl": "https://github.com/acme/app/releases/download/v2.0.1/app.lpk", "sha256": strings.Repeat("b", 64),
				})
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				if got := store.server.db.AppVersion.Query().Where(appversion.AppIDEQ(externalApp.ID)).CountX(ctx); got != 2 {
					t.Fatalf("URL app version count = %d, want 2", got)
				}

				response = store.doMultipart(http.MethodPost, "/api/v1/apps/"+strconv.Itoa(record.ID)+"/screenshots", "file", "screen.png", testPNG(t, 2, 2), nil)
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				response = store.doMultipart(http.MethodPost, "/api/v1/me/avatar", "file", "avatar.png", testPNG(t, 2, 2), nil)
				assertPackagePolicyStatus(t, response, http.StatusOK)
				response = store.do(http.MethodGet, localVersion.DownloadURL, nil)
				assertPackagePolicyStatus(t, response, http.StatusOK)
				if !bytes.Equal(response.Body.Bytes(), lpk) {
					t.Fatal("existing package download content changed while uploads were disabled")
				}

				if err := store.server.setSetting(ctx, settingAllowPackageUpload, "true"); err != nil {
					t.Fatal(err)
				}
				response = store.doMultipart(http.MethodPost, versionPath, "file", "app.lpk", lpk, map[string]string{"version": "1.0.2"})
				assertPackagePolicyStatus(t, response, http.StatusCreated)
				reenabledLPK := testLPKArchive(t, "cloud.lazycat.test.reenabled", "1.0.0", "Reenabled", "Uploads reenabled")
				response = store.doMultipart(http.MethodPost, "/api/v1/apps", "file", "reenabled.lpk", reenabledLPK, nil)
				assertPackagePolicyStatus(t, response, http.StatusCreated)
			})
		}
	}
}

type packagePolicyUnreadBody struct {
	reads int
}

func (b *packagePolicyUnreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, io.ErrUnexpectedEOF
}

func packagePolicyRecordCounts(t *testing.T, store *testApp) [4]int {
	t.Helper()
	ctx := t.Context()
	return [4]int{
		store.server.db.App.Query().CountX(ctx),
		store.server.db.AppVersion.Query().CountX(ctx),
		store.server.db.ReviewRequest.Query().CountX(ctx),
		store.server.db.Asset.Query().CountX(ctx),
	}
}

func assertPackagePolicyStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, want, response.Body.String())
	}
}

func assertPackagePolicyDenied(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	assertPackagePolicyStatus(t, response, http.StatusForbidden)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "PACKAGE_UPLOAD_DISABLED" {
		t.Fatalf("error code = %q, want PACKAGE_UPLOAD_DISABLED", body.Error.Code)
	}
}
