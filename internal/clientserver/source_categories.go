package clientserver

import (
	"encoding/json"
	"fmt"
	"strings"

	"lazycat.community/appstore/internal/catalogmeta"
)

func normalizeSourceCategories(input []SourceCategoryDTO) []SourceCategoryDTO {
	if len(input) == 0 {
		return nil
	}
	seen := map[int]struct{}{}
	out := make([]SourceCategoryDTO, 0, len(input))
	for _, item := range input {
		if item.ID <= 0 {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		nameI18n := catalogmeta.CleanLocalizedText(catalogmeta.LocalizedText(item.NameI18n))
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = nameI18n.Fallback("")
		}
		if name == "" {
			continue
		}
		item.Name = name
		item.NameI18n = nameI18n
		item.Slug = strings.TrimSpace(item.Slug)
		if item.Slug == "" {
			item.Slug = fmt.Sprintf("category-%d", item.ID)
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}

	valid := map[int]SourceCategoryDTO{}
	for _, item := range out {
		valid[item.ID] = item
	}
	for index := range out {
		parentID := out[index].ParentID
		if parentID == nil {
			continue
		}
		parent, ok := valid[*parentID]
		if !ok || parent.ID == out[index].ID || parent.ParentID != nil {
			out[index].ParentID = nil
		}
	}
	return out
}

func encodeSourceCategories(categories []SourceCategoryDTO) string {
	categories = normalizeSourceCategories(categories)
	if len(categories) == 0 {
		return ""
	}
	raw, err := json.Marshal(categories)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeSourceCategories(raw string) []SourceCategoryDTO {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var categories []SourceCategoryDTO
	if err := json.Unmarshal([]byte(raw), &categories); err != nil {
		return nil
	}
	return normalizeSourceCategories(categories)
}

func sourceCategoryIDs(categories []SourceCategoryDTO) map[int]struct{} {
	categories = normalizeSourceCategories(categories)
	ids := make(map[int]struct{}, len(categories))
	for _, category := range categories {
		ids[category.ID] = struct{}{}
	}
	return ids
}
