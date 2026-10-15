package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/app"
	"lazycat.community/appstore/ent/category"
	"lazycat.community/appstore/ent/mcptoken"
	"lazycat.community/appstore/internal/catalogmeta"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpCategoriesOutput struct {
	Categories []categoryDTO `json:"categories"`
}

type mcpCategoryOutput struct {
	Category categoryDTO `json:"category"`
}

type mcpCreateCategoryInput struct {
	Name      string                    `json:"name,omitempty" jsonschema:"category name; optional when nameI18n has a name"`
	NameI18n  catalogmeta.LocalizedText `json:"nameI18n,omitempty" jsonschema:"localized names keyed by language, such as zh-CN and en"`
	Slug      string                    `json:"slug,omitempty" jsonschema:"unique category slug; generated from name when omitted"`
	ParentID  *int                      `json:"parentId,omitempty" jsonschema:"optional parent category ID; maximum depth is two levels"`
	SortOrder *int                      `json:"sortOrder,omitempty" jsonschema:"optional display order"`
}

type mcpUpdateCategoryInput struct {
	ID          int                       `json:"id" jsonschema:"category ID to update"`
	Name        string                    `json:"name,omitempty" jsonschema:"replacement category name"`
	NameI18n    catalogmeta.LocalizedText `json:"nameI18n,omitempty" jsonschema:"replacement localized names keyed by language"`
	Slug        string                    `json:"slug,omitempty" jsonschema:"replacement unique slug"`
	ParentID    *int                      `json:"parentId,omitempty" jsonschema:"new parent category ID"`
	ClearParent bool                      `json:"clearParent,omitempty" jsonschema:"set true to make this a top-level category"`
	SortOrder   *int                      `json:"sortOrder,omitempty" jsonschema:"replacement display order"`
}

type mcpDeleteCategoryInput struct {
	ID int `json:"id" jsonschema:"category ID to delete"`
}

func (s *Server) requireMCPAdmin(ctx context.Context, req *mcp.CallToolRequest) error {
	_, principalType, err := s.mcpEffectiveUser(ctx, req)
	if err != nil {
		return err
	}
	if principalType != mcptoken.PrincipalTypeADMIN {
		return errors.New("admin MCP token is required")
	}
	return nil
}

func (s *Server) mcpAdminListCategories(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, mcpCategoriesOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpCategoriesOutput{}, err
	}
	records, err := s.db.Category.Query().Order(entgo.Asc(category.FieldSortOrder), entgo.Asc(category.FieldName)).All(ctx)
	if err != nil {
		return nil, mcpCategoriesOutput{}, err
	}
	out := make([]categoryDTO, 0, len(records))
	for _, record := range records {
		out = append(out, toCategoryDTO(record))
	}
	return nil, mcpCategoriesOutput{Categories: out}, nil
}

func (s *Server) mcpAdminCreateCategory(ctx context.Context, req *mcp.CallToolRequest, input mcpCreateCategoryInput) (*mcp.CallToolResult, mcpCategoryOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpCategoryOutput{}, err
	}
	localized := catalogmeta.CleanLocalizedText(input.NameI18n)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = localized.Fallback("")
	}
	if name == "" {
		return nil, mcpCategoryOutput{}, errors.New("category name is required")
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	if err := s.validateCategoryParent(ctx, 0, input.ParentID); err != nil {
		return nil, mcpCategoryOutput{}, err
	}
	create := s.db.Category.Create().SetName(name).SetNameI18n(catalogmeta.EncodeLocalizedText(localized)).SetSlug(slug)
	if input.ParentID != nil {
		create.SetParentID(*input.ParentID)
	}
	if input.SortOrder != nil {
		create.SetSortOrder(*input.SortOrder)
	}
	record, err := create.Save(ctx)
	if err != nil {
		return nil, mcpCategoryOutput{}, fmt.Errorf("create category: %w", err)
	}
	s.invalidateSourceFeed()
	return nil, mcpCategoryOutput{Category: toCategoryDTO(record)}, nil
}

func (s *Server) mcpAdminUpdateCategory(ctx context.Context, req *mcp.CallToolRequest, input mcpUpdateCategoryInput) (*mcp.CallToolResult, mcpCategoryOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpCategoryOutput{}, err
	}
	if input.ID <= 0 {
		return nil, mcpCategoryOutput{}, errors.New("category ID is required")
	}
	if input.ParentID != nil && input.ClearParent {
		return nil, mcpCategoryOutput{}, errors.New("parentId and clearParent cannot be used together")
	}
	update := s.db.Category.UpdateOneID(input.ID)
	if name := strings.TrimSpace(input.Name); name != "" {
		update.SetName(name)
	}
	if input.NameI18n != nil {
		localized := catalogmeta.CleanLocalizedText(input.NameI18n)
		update.SetNameI18n(catalogmeta.EncodeLocalizedText(localized))
		if strings.TrimSpace(input.Name) == "" {
			if name := localized.Fallback(""); name != "" {
				update.SetName(name)
			}
		}
	}
	if slug := strings.TrimSpace(input.Slug); slug != "" {
		update.SetSlug(slug)
	}
	if input.ParentID != nil {
		if err := s.validateCategoryParent(ctx, input.ID, input.ParentID); err != nil {
			return nil, mcpCategoryOutput{}, err
		}
		update.SetParentID(*input.ParentID)
	} else if input.ClearParent {
		update.ClearParentID()
	}
	if input.SortOrder != nil {
		update.SetSortOrder(*input.SortOrder)
	}
	record, err := update.Save(ctx)
	if err != nil {
		return nil, mcpCategoryOutput{}, fmt.Errorf("update category: %w", err)
	}
	s.invalidateSourceFeed()
	return nil, mcpCategoryOutput{Category: toCategoryDTO(record)}, nil
}

func (s *Server) mcpAdminDeleteCategory(ctx context.Context, req *mcp.CallToolRequest, input mcpDeleteCategoryInput) (*mcp.CallToolResult, okResponse, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, okResponse{}, err
	}
	if input.ID <= 0 {
		return nil, okResponse{}, errors.New("category ID is required")
	}
	if _, err := s.db.Category.Get(ctx, input.ID); err != nil {
		return nil, okResponse{}, err
	}
	children, err := s.db.Category.Query().Where(category.ParentIDEQ(input.ID)).Count(ctx)
	if err != nil {
		return nil, okResponse{}, err
	}
	if children > 0 {
		return nil, okResponse{}, fmt.Errorf("category has %d child categories", children)
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, okResponse{}, err
	}
	defer tx.Rollback()
	if _, err := tx.App.Update().Where(app.CategoryIDEQ(input.ID)).ClearCategoryID().Save(ctx); err != nil {
		return nil, okResponse{}, err
	}
	if err := tx.Category.DeleteOneID(input.ID).Exec(ctx); err != nil {
		return nil, okResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return nil, okResponse{}, err
	}
	s.invalidateSourceFeed()
	return nil, okResponse{OK: true}, nil
}
