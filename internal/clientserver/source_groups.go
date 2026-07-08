package clientserver

import (
	"encoding/json"
	"net/url"
	"strings"
)

type SourceGroupDTO struct {
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}

func normalizeGroupCode(value string) string {
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) != 6 {
		return ""
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return code
}

func normalizeGroupCodes(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		code := normalizeGroupCode(value)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func encodeStringSlice(values []string) string {
	values = normalizeGroupCodes(values)
	if len(values) == 0 {
		return ""
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeStringSlice(raw string) []string {
	var values []string
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return normalizeGroupCodes(values)
}

func encodeSourceGroups(groups []SourceGroupDTO) string {
	if len(groups) == 0 {
		return ""
	}
	out := make([]SourceGroupDTO, 0, len(groups))
	seen := map[string]struct{}{}
	for _, group := range groups {
		name := strings.TrimSpace(group.Name)
		code := normalizeGroupCode(group.Code)
		key := name + "\x00" + code
		if name == "" || key == "\x00" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, SourceGroupDTO{Name: name, Code: code})
	}
	if len(out) == 0 {
		return ""
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeSourceGroups(raw string) []SourceGroupDTO {
	var groups []SourceGroupDTO
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		return nil
	}
	out := make([]SourceGroupDTO, 0, len(groups))
	for _, group := range groups {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			continue
		}
		out = append(out, SourceGroupDTO{Name: name, Code: normalizeGroupCode(group.Code)})
	}
	return out
}

func removeInvalidGroupCodes(current, invalid []string) []string {
	current = normalizeGroupCodes(current)
	invalid = normalizeGroupCodes(invalid)
	if len(current) == 0 || len(invalid) == 0 {
		return current
	}
	invalidSet := map[string]struct{}{}
	for _, code := range invalid {
		invalidSet[code] = struct{}{}
	}
	out := make([]string, 0, len(current))
	for _, code := range current {
		if _, bad := invalidSet[code]; !bad {
			out = append(out, code)
		}
	}
	return out
}

func withGroupCodes(rawURL string, codes []string) string {
	codes = normalizeGroupCodes(codes)
	if len(codes) == 0 {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	value := strings.Join(codes, ",")
	if err != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		return rawURL + separator + "groupCodes=" + url.QueryEscape(value)
	}
	q := parsed.Query()
	q.Set("groupCodes", value)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
