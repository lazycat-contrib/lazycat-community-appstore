package clientserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	"gitee.com/linakesi/lzc-sdk/lang/go/common"
	"gitee.com/linakesi/lzc-sdk/lang/go/localdevice"
	"google.golang.org/grpc"
)

type SystemNotifier interface {
	NotifyAutoUpdate(ctx context.Context, userID string, completedAt time.Time, apps []string, failed int) error
}

type notificationContent struct {
	Title string
	Body  string
}

type lazyCatSystemNotifier struct{}
type unavailableSystemNotifier struct{}

func NewLazyCatSystemNotifier() SystemNotifier { return lazyCatSystemNotifier{} }

func (unavailableSystemNotifier) NotifyAutoUpdate(context.Context, string, time.Time, []string, int) error {
	return nil
}

func successfulAutoUpdateApps(result UpdateQueueResultDTO) ([]string, int) {
	apps := make([]string, 0, len(result.Items))
	failed := 0
	for _, item := range result.Items {
		switch item.Status {
		case "success":
			name := sanitizeNotificationAppName(item.AppName)
			if name == "" {
				name = sanitizeNotificationAppName(item.PackageID)
			}
			if name != "" {
				apps = append(apps, name)
			}
		case "failed":
			failed++
		}
	}
	return apps, failed
}

func autoUpdateNotification(language string, location *time.Location, completedAt time.Time, apps []string, failed int) notificationContent {
	if location == nil {
		location = time.UTC
	}
	completedAt = completedAt.In(location)
	if len(apps) == 0 && failed > 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh") {
			return notificationContent{
				Title: "自动更新失败",
				Body:  fmt.Sprintf("%s 自动更新未完成，%d 个软件更新失败。", completedAt.Format("01月02日 15:04"), failed),
			}
		}
		return notificationContent{
			Title: "Automatic updates failed",
			Body:  fmt.Sprintf("At %s, automatic updates did not complete; %d %s failed.", completedAt.Format("Jan 2, 3:04 PM"), failed, plural(failed, "update", "updates")),
		}
	}
	displayedApps, omitted := notificationAppNames(apps)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh") {
		appList := strings.Join(displayedApps, "、")
		if omitted > 0 {
			appList += fmt.Sprintf("，另有 %d 个", omitted)
		}
		body := fmt.Sprintf("%s 自动更新了 %d 个软件：%s。", completedAt.Format("01月02日 15:04"), len(apps), appList)
		if failed > 0 {
			body += fmt.Sprintf("另有 %d 个更新失败。", failed)
		}
		return notificationContent{Title: "自动更新已完成", Body: body}
	}
	appList := joinEnglish(displayedApps)
	if omitted > 0 {
		appList += fmt.Sprintf(", plus %d more", omitted)
	}
	body := fmt.Sprintf("At %s, %d %s updated: %s.", completedAt.Format("Jan 2, 3:04 PM"), len(apps), plural(len(apps), "app was", "apps were"), appList)
	if failed > 0 {
		body += fmt.Sprintf(" %d %s failed.", failed, plural(failed, "update", "updates"))
	}
	return notificationContent{Title: "Automatic updates completed", Body: body}
}

func notificationAppNames(apps []string) ([]string, int) {
	const maxNames = 8
	displayed := make([]string, 0, min(len(apps), maxNames))
	for _, app := range apps {
		if name := sanitizeNotificationAppName(app); name != "" {
			displayed = append(displayed, name)
		}
		if len(displayed) == maxNames {
			break
		}
	}
	return displayed, max(0, len(apps)-len(displayed))
}

func sanitizeNotificationAppName(value string) string {
	const maxRunes = 64
	runes := make([]rune, 0, min(len(value), maxRunes))
	for _, r := range value {
		if unicode.IsControl(r) || isBidirectionalControl(r) {
			continue
		}
		runes = append(runes, r)
		if len(runes) == maxRunes {
			break
		}
	}
	return strings.Join(strings.Fields(string(runes)), " ")
}

func isBidirectionalControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069'
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func joinEnglish(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
	}
}

func (lazyCatSystemNotifier) NotifyAutoUpdate(ctx context.Context, userID string, completedAt time.Time, apps []string, failed int) error {
	request, err := listEndDevicesRequest(userID)
	if err != nil {
		return err
	}
	listCtx, cancelList := context.WithTimeout(lazycatContext(ctx, userID), 10*time.Second)
	defer cancelList()
	gw, err := gohelper.NewAPIGateway(listCtx)
	if err != nil {
		return err
	}
	defer func() { _ = gw.Close() }()
	devices, err := gw.Devices.ListEndDevices(listCtx, request)
	if err != nil {
		return err
	}
	var notifyErrors []error
	var notifyErrorsMu sync.Mutex
	var notifyGroup sync.WaitGroup
	for _, device := range devices.GetDevices() {
		if !device.GetIsOnline() || strings.TrimSpace(device.GetDeviceApiUrl()) == "" {
			continue
		}
		notifyGroup.Go(func() {
			deviceCtx, cancelDevice := context.WithTimeout(ctx, 8*time.Second)
			defer cancelDevice()
			location := time.UTC
			if candidate, loadErr := time.LoadLocation(strings.TrimSpace(device.GetTimeZone())); loadErr == nil {
				location = candidate
			}
			content := autoUpdateNotification(device.GetLang(), location, completedAt, apps, failed)
			if err := notifyLazyCatDevice(deviceCtx, device.GetDeviceApiUrl(), content); err != nil {
				notifyErrorsMu.Lock()
				notifyErrors = append(notifyErrors, fmt.Errorf("notify device %s: %w", device.GetUniqueDeivceId(), err))
				notifyErrorsMu.Unlock()
			}
		})
	}
	notifyGroup.Wait()
	return errors.Join(notifyErrors...)
}

func listEndDevicesRequest(userID string) (*common.ListEndDeviceRequest, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" || userID == clientIdentityLocal {
		return nil, errors.New("a LazyCat user ID is required for system notifications")
	}
	return &common.ListEndDeviceRequest{Uid: userID}, nil
}

type deviceAPICredentials struct {
	host       string
	dialOption grpc.DialOption

	mu    sync.Mutex
	token *gohelper.AuthToken
}

func (c *deviceAPICredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == nil || !time.Now().Before(c.token.Deadline) {
		conn, err := grpc.NewClient(c.host, c.dialOption)
		if err != nil {
			return nil, err
		}
		defer func() { _ = conn.Close() }()
		c.token, err = gohelper.RequestAuthToken(ctx, conn)
		if err != nil {
			return nil, err
		}
	}
	return map[string]string{"lzc_dapi_auth_token": c.token.Token}, nil
}

func (*deviceAPICredentials) RequireTransportSecurity() bool { return true }

func notifyLazyCatDevice(ctx context.Context, apiURL string, content notificationContent) error {
	parsed, err := url.Parse(apiURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid device API URL %q", apiURL)
	}
	dialOption, err := gohelper.BuildClientCredOption(gohelper.CAPath, gohelper.APPKeyPath, gohelper.APPCertPath)
	if err != nil {
		return err
	}
	credentials := &deviceAPICredentials{host: parsed.Host, dialOption: dialOption}
	conn, err := grpc.NewClient(parsed.Host, dialOption, grpc.WithPerRPCCredentials(credentials))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_, err = localdevice.NewNotificationServiceClient(conn).Notify(ctx, &localdevice.NotifyRequest{Title: content.Title, Body: content.Body})
	return err
}

var _ SystemNotifier = lazyCatSystemNotifier{}
var _ SystemNotifier = unavailableSystemNotifier{}
