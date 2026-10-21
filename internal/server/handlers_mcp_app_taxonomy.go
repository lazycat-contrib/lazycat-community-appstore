package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/tag"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpSetAppCategoryInput struct {
	AppID         int    `json:"appId,omitempty" jsonschema:"app ID; provide this or packageId"`
	PackageID     string `json:"packageId,omitempty" jsonschema:"app package ID; provide this or appId"`
	CategoryID    int    `json:"categoryId,omitempty" jsonschema:"existing category ID to assign"`
	ClearCategory bool   `json:"clearCategory,omitempty" jsonschema:"set true to make the app uncategorized"`
}

type mcpSetAppTagsInput struct {
	AppID     int    `json:"appId,omitempty" jsonschema:"app ID; provide this or packageId"`
	PackageID string `json:"packageId,omitempty" jsonschema:"app package ID; provide this or appId"`
	TagIDs    []int  `json:"tagIds" jsonschema:"complete list of existing tag IDs; an empty array removes all tags"`
}

type mcpAppTagInput struct {
	AppID     int    `json:"appId,omitempty" jsonschema:"app ID; provide this or packageId"`
	PackageID string `json:"packageId,omitempty" jsonschema:"app package ID; provide this or appId"`
	TagID     int    `json:"tagId" jsonschema:"existing tag ID to add or remove"`
}

type mcpAppTaxonomyOutput struct {
	AppID      int      `json:"appId"`
	PackageID  string   `json:"packageId"`
	CategoryID *int     `json:"categoryId"`
	Category   string   `json:"category"`
	TagIDs     []int    `json:"tagIds"`
	Tags       []string `json:"tags"`
}

func (s *Server) mcpAdminSetAppCategory(ctx context.Context, req *mcp.CallToolRequest, input mcpSetAppCategoryInput) (*mcp.CallToolResult, mcpAppTaxonomyOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	record, err := s.mcpTaxonomyApp(ctx, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	if input.CategoryID < 0 || (input.ClearCategory && input.CategoryID != 0) || (!input.ClearCategory && input.CategoryID == 0) {
		return nil, mcpAppTaxonomyOutput{}, errors.New("provide categoryId or clearCategory, but not both")
	}
	update := s.db.App.UpdateOneID(record.ID)
	if input.ClearCategory {
		update.ClearCategoryID()
	} else {
		if _, err := s.db.Category.Get(ctx, input.CategoryID); err != nil {
			return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("category not found: %w", err)
		}
		update.SetCategoryID(input.CategoryID)
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("set app category: %w", err)
	}
	s.invalidateSourceFeed()
	out, err := s.mcpAppTaxonomy(ctx, updated)
	return nil, out, err
}

func (s *Server) mcpAdminSetAppTags(ctx context.Context, req *mcp.CallToolRequest, input mcpSetAppTagsInput) (*mcp.CallToolResult, mcpAppTaxonomyOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	record, err := s.mcpTaxonomyApp(ctx, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	if input.TagIDs == nil {
		return nil, mcpAppTaxonomyOutput{}, errors.New("tagIds is required; use an empty array to clear tags")
	}
	ids := slices.Clone(input.TagIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	for _, id := range ids {
		if id <= 0 {
			return nil, mcpAppTaxonomyOutput{}, errors.New("tag IDs must be positive")
		}
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if len(ids) > 0 {
		count, err := tx.Tag.Query().Where(tag.IDIn(ids...)).Count(ctx)
		if err != nil {
			return nil, mcpAppTaxonomyOutput{}, err
		}
		if count != len(ids) {
			return nil, mcpAppTaxonomyOutput{}, errors.New("one or more tag IDs do not exist")
		}
	}
	if _, err := tx.AppTag.Delete().Where(apptag.AppIDEQ(record.ID)).Exec(ctx); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	for _, id := range ids {
		if _, err := tx.AppTag.Create().SetAppID(record.ID).SetTagID(id).Save(ctx); err != nil {
			return nil, mcpAppTaxonomyOutput{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	s.invalidateSourceFeed()
	out, err := s.mcpAppTaxonomy(ctx, record)
	return nil, out, err
}

func (s *Server) mcpAdminAddAppTag(ctx context.Context, req *mcp.CallToolRequest, input mcpAppTagInput) (*mcp.CallToolResult, mcpAppTaxonomyOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	record, err := s.mcpTaxonomyApp(ctx, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	if input.TagID <= 0 {
		return nil, mcpAppTaxonomyOutput{}, errors.New("tag ID is required")
	}
	if _, err := s.db.Tag.Get(ctx, input.TagID); err != nil {
		return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("tag not found: %w", err)
	}
	_, err = s.db.AppTag.Create().SetAppID(record.ID).SetTagID(input.TagID).Save(ctx)
	if err != nil {
		if !entgo.IsConstraintError(err) {
			return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("add app tag: %w", err)
		}
		exists, checkErr := s.db.AppTag.Query().Where(apptag.AppIDEQ(record.ID), apptag.TagIDEQ(input.TagID)).Exist(ctx)
		if checkErr != nil {
			return nil, mcpAppTaxonomyOutput{}, checkErr
		}
		if !exists {
			return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("add app tag: %w", err)
		}
	}
	s.invalidateSourceFeed()
	out, err := s.mcpAppTaxonomy(ctx, record)
	return nil, out, err
}

func (s *Server) mcpAdminRemoveAppTag(ctx context.Context, req *mcp.CallToolRequest, input mcpAppTagInput) (*mcp.CallToolResult, mcpAppTaxonomyOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	record, err := s.mcpTaxonomyApp(ctx, input.AppID, input.PackageID)
	if err != nil {
		return nil, mcpAppTaxonomyOutput{}, err
	}
	if input.TagID <= 0 {
		return nil, mcpAppTaxonomyOutput{}, errors.New("tag ID is required")
	}
	if _, err := s.db.AppTag.Delete().Where(apptag.AppIDEQ(record.ID), apptag.TagIDEQ(input.TagID)).Exec(ctx); err != nil {
		return nil, mcpAppTaxonomyOutput{}, fmt.Errorf("remove app tag: %w", err)
	}
	s.invalidateSourceFeed()
	out, err := s.mcpAppTaxonomy(ctx, record)
	return nil, out, err
}

func (s *Server) mcpTaxonomyApp(ctx context.Context, appID int, packageID string) (*entgo.App, error) {
	if appID > 0 && strings.TrimSpace(packageID) != "" {
		return nil, errors.New("provide appId or packageId, but not both")
	}
	return s.mcpFindApp(ctx, appID, packageID)
}

func (s *Server) mcpAppTaxonomy(ctx context.Context, record *entgo.App) (mcpAppTaxonomyOutput, error) {
	out := mcpAppTaxonomyOutput{
		AppID:      record.ID,
		PackageID:  record.PackageID,
		CategoryID: record.CategoryID,
		TagIDs:     []int{},
		Tags:       []string{},
	}
	if record.CategoryID != nil {
		category, err := s.db.Category.Get(ctx, *record.CategoryID)
		if err != nil {
			return mcpAppTaxonomyOutput{}, err
		}
		out.Category = category.Name
	}
	links, err := s.db.AppTag.Query().Where(apptag.AppIDEQ(record.ID)).All(ctx)
	if err != nil {
		return mcpAppTaxonomyOutput{}, err
	}
	if len(links) == 0 {
		return out, nil
	}
	ids := make([]int, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.TagID)
	}
	slices.Sort(ids)
	records, err := s.db.Tag.Query().Where(tag.IDIn(ids...)).All(ctx)
	if err != nil {
		return mcpAppTaxonomyOutput{}, err
	}
	byID := make(map[int]string, len(records))
	for _, tag := range records {
		byID[tag.ID] = tag.Name
	}
	for _, id := range ids {
		if name, ok := byID[id]; ok {
			out.TagIDs = append(out.TagIDs, id)
			out.Tags = append(out.Tags, name)
		}
	}
	return out, nil
}
