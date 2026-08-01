package clientserver

import (
	"strings"
	"testing"
	"time"
)

func TestAutoUpdateNotificationLocalizesCompletionAndApps(t *testing.T) {
	completedAt := time.Date(2026, time.August, 1, 14, 30, 0, 0, time.UTC)

	zh := autoUpdateNotification("zh-Hans", time.UTC, completedAt, []string{"Jellyfin", "Vaultwarden"}, 1)
	if zh.Title != "自动更新已完成" || !strings.Contains(zh.Body, "08月01日 14:30") || !strings.Contains(zh.Body, "Jellyfin、Vaultwarden") || !strings.Contains(zh.Body, "1 个更新失败") {
		t.Fatalf("unexpected Chinese notification: %#v", zh)
	}

	en := autoUpdateNotification("en-US", time.UTC, completedAt, []string{"Jellyfin", "Vaultwarden"}, 1)
	if en.Title != "Automatic updates completed" || !strings.Contains(en.Body, "Aug 1, 2:30 PM") || !strings.Contains(en.Body, "Jellyfin and Vaultwarden") || !strings.Contains(en.Body, "1 update failed") {
		t.Fatalf("unexpected English notification: %#v", en)
	}
}

func TestAutoUpdateNotificationReportsCompleteFailure(t *testing.T) {
	completedAt := time.Date(2026, time.August, 1, 14, 30, 0, 0, time.UTC)
	zh := autoUpdateNotification("zh-Hans", time.UTC, completedAt, nil, 2)
	if zh.Title != "自动更新失败" || !strings.Contains(zh.Body, "2 个软件更新失败") {
		t.Fatalf("unexpected Chinese failure notification: %#v", zh)
	}
	en := autoUpdateNotification("en", time.UTC, completedAt, nil, 2)
	if en.Title != "Automatic updates failed" || !strings.Contains(en.Body, "2 updates failed") {
		t.Fatalf("unexpected English failure notification: %#v", en)
	}
}

func TestSuccessfulAutoUpdateAppsOnlyIncludesCompletedItems(t *testing.T) {
	apps, failed := successfulAutoUpdateApps(UpdateQueueResultDTO{Items: []UpdateQueueItemDTO{
		{AppName: "Updated", Status: "success"},
		{AppName: "Broken", Status: "failed"},
		{AppName: "Cancelled", Status: "cancelled"},
	}})
	if len(apps) != 1 || apps[0] != "Updated" || failed != 1 {
		t.Fatalf("apps=%v failed=%d", apps, failed)
	}
}

func TestListEndDevicesRequestScopesNotificationToUser(t *testing.T) {
	request, err := listEndDevicesRequest(" alice ")
	if err != nil || request.GetUid() != "alice" {
		t.Fatalf("request=%#v err=%v", request, err)
	}
	for _, userID := range []string{"", "local"} {
		if _, err := listEndDevicesRequest(userID); err == nil {
			t.Fatalf("user %q should not produce a device request", userID)
		}
	}
}

func TestNotificationAppNamesRemoveControlsAndLimitContent(t *testing.T) {
	unsafeName := "Safe\n\u202eSpoofed\tName"
	if got := sanitizeNotificationAppName(unsafeName); got != "SafeSpoofedName" {
		t.Fatalf("sanitized name = %q", got)
	}
	longName := strings.Repeat("猫", 100)
	if got := []rune(sanitizeNotificationAppName(longName)); len(got) != 64 {
		t.Fatalf("sanitized name length = %d", len(got))
	}
	names, omitted := notificationAppNames([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"})
	if len(names) != 8 || omitted != 1 {
		t.Fatalf("names=%v omitted=%d", names, omitted)
	}
}
