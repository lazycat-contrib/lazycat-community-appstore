package clientserver

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func normalizeSourceAnnouncements(input []SourceAnnouncementDTO) []SourceAnnouncementDTO {
	if len(input) == 0 {
		return nil
	}
	out := make([]SourceAnnouncementDTO, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		item.Title = strings.TrimSpace(item.Title)
		item.Body = strings.TrimSpace(item.Body)
		if !item.Enabled || (item.Title == "" && item.Body == "") {
			continue
		}
		item.Level = strings.ToLower(strings.TrimSpace(item.Level))
		switch item.Level {
		case "info", "warning", "success":
		default:
			item.Level = "info"
		}
		item.LinkLabel = strings.TrimSpace(item.LinkLabel)
		item.LinkURL = strings.TrimSpace(item.LinkURL)
		item.StartsAt = normalizeAnnouncementTimeString(item.StartsAt)
		item.EndsAt = normalizeAnnouncementTimeString(item.EndsAt)
		item.UpdatedAt = normalizeAnnouncementTimeString(item.UpdatedAt)
		key := announcementCacheKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func encodeSourceAnnouncements(announcements []SourceAnnouncementDTO) string {
	announcements = normalizeSourceAnnouncements(announcements)
	if len(announcements) == 0 {
		return ""
	}
	raw, err := json.Marshal(announcements)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeSourceAnnouncements(raw string) []SourceAnnouncementDTO {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var announcements []SourceAnnouncementDTO
	if err := json.Unmarshal([]byte(raw), &announcements); err != nil {
		return nil
	}
	return normalizeSourceAnnouncements(announcements)
}

func normalizeAnnouncementTimeString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func announcementCacheKey(item SourceAnnouncementDTO) string {
	if item.ID > 0 {
		return "id:" + strconv.Itoa(item.ID)
	}
	return item.Level + "\x00" + item.Title + "\x00" + item.Body + "\x00" + item.UpdatedAt
}
