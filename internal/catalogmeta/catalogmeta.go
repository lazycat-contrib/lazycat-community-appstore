package catalogmeta

import (
	"encoding/json"
	"strings"
	"time"
)

type LocalizedText map[string]string

type Screenshot struct {
	ID         int       `json:"id,omitempty"`
	AppID      int       `json:"appId,omitempty"`
	ImageURL   string    `json:"imageUrl"`
	Caption    string    `json:"caption,omitempty"`
	DeviceType string    `json:"deviceType"`
	SortOrder  int       `json:"sortOrder,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
}

const (
	DeviceDesktop = "DESKTOP"
	DeviceMobile  = "MOBILE"
)

func CleanLocalizedText(value LocalizedText) LocalizedText {
	out := LocalizedText{}
	for key, item := range value {
		key = strings.TrimSpace(key)
		item = strings.TrimSpace(item)
		if key != "" && item != "" {
			out[key] = item
		}
	}
	return out
}

func (value LocalizedText) IsZero() bool {
	return len(CleanLocalizedText(value)) == 0
}

func (value LocalizedText) Fallback(defaultValue string) string {
	value = CleanLocalizedText(value)
	for _, key := range []string{"zh-CN", "zh_Hans", "zh", "en"} {
		if item := value[key]; item != "" {
			return item
		}
	}
	for _, item := range value {
		if item != "" {
			return item
		}
	}
	return strings.TrimSpace(defaultValue)
}

func EncodeLocalizedText(value LocalizedText) string {
	value = CleanLocalizedText(value)
	if value.IsZero() {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func DecodeLocalizedText(raw string) LocalizedText {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return LocalizedText{}
	}
	var value LocalizedText
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return LocalizedText{}
	}
	return CleanLocalizedText(value)
}

func CleanDeviceType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case DeviceMobile:
		return DeviceMobile
	default:
		return DeviceDesktop
	}
}

func EncodeScreenshots(screenshots []Screenshot) string {
	if len(screenshots) == 0 {
		return ""
	}
	normalized := make([]Screenshot, 0, len(screenshots))
	for _, item := range screenshots {
		item.ImageURL = strings.TrimSpace(item.ImageURL)
		if item.ImageURL == "" {
			continue
		}
		item.Caption = strings.TrimSpace(item.Caption)
		item.DeviceType = CleanDeviceType(item.DeviceType)
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return ""
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func DecodeScreenshots(raw string) []Screenshot {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var screenshots []Screenshot
	if err := json.Unmarshal([]byte(raw), &screenshots); err != nil {
		return nil
	}
	out := make([]Screenshot, 0, len(screenshots))
	for _, item := range screenshots {
		item.ImageURL = strings.TrimSpace(item.ImageURL)
		if item.ImageURL == "" {
			continue
		}
		item.Caption = strings.TrimSpace(item.Caption)
		item.DeviceType = CleanDeviceType(item.DeviceType)
		out = append(out, item)
	}
	return out
}
