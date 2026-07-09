package clientserver

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

func normalizeSourceAds(input []SourceAdDTO) []SourceAdDTO {
	if len(input) == 0 {
		return nil
	}
	out := make([]SourceAdDTO, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		item.Title = strings.TrimSpace(item.Title)
		item.Body = strings.TrimSpace(item.Body)
		item.ImageURL = cleanHTTPURL(item.ImageURL)
		if !item.Enabled || (item.Title == "" && item.Body == "" && item.ImageURL == "") {
			continue
		}
		item.LinkLabel = strings.TrimSpace(item.LinkLabel)
		item.LinkURL = cleanHTTPURL(item.LinkURL)
		item.StartsAt = normalizeAnnouncementTimeString(item.StartsAt)
		item.EndsAt = normalizeAnnouncementTimeString(item.EndsAt)
		item.UpdatedAt = normalizeAnnouncementTimeString(item.UpdatedAt)
		key := adCacheKey(item)
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

func encodeSourceAds(ads []SourceAdDTO) string {
	ads = normalizeSourceAds(ads)
	if len(ads) == 0 {
		return ""
	}
	raw, err := json.Marshal(ads)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeSourceAds(raw string) []SourceAdDTO {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ads []SourceAdDTO
	if err := json.Unmarshal([]byte(raw), &ads); err != nil {
		return nil
	}
	return normalizeSourceAds(ads)
}

func cleanHTTPURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return value
}

func adCacheKey(item SourceAdDTO) string {
	if item.ID > 0 {
		return "id:" + strconv.Itoa(item.ID)
	}
	return item.Title + "\x00" + item.Body + "\x00" + item.ImageURL + "\x00" + item.UpdatedAt
}
