package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/apptag"
	"lazycat.community/appstore/ent/tag"
	"lazycat.community/appstore/internal/catalogmeta"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpTagsOutput struct {
	Tags []tagDTO `json:"tags"`
}

type mcpTagOutput struct {
	Tag tagDTO `json:"tag"`
}

type mcpCreateTagInput struct {
	Name     string                    `json:"name,omitempty" jsonschema:"tag name; optional when nameI18n has a name"`
	NameI18n catalogmeta.LocalizedText `json:"nameI18n,omitempty" jsonschema:"localized names keyed by language, such as zh-CN and en"`
	Slug     string                    `json:"slug,omitempty" jsonschema:"unique tag slug; generated from name when omitted"`
}

type mcpUpdateTagInput struct {
	ID       int                       `json:"id" jsonschema:"tag ID to update"`
	Name     string                    `json:"name,omitempty" jsonschema:"replacement tag name"`
	NameI18n catalogmeta.LocalizedText `json:"nameI18n,omitempty" jsonschema:"replacement localized names keyed by language"`
	Slug     string                    `json:"slug,omitempty" jsonschema:"replacement unique slug"`
}

type mcpDeleteTagInput struct {
	ID int `json:"id" jsonschema:"tag ID to delete"`
}

func (s *Server) mcpAdminListTags(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, mcpTagsOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpTagsOutput{}, err
	}
	records, err := s.db.Tag.Query().Order(entgo.Asc(tag.FieldName)).All(ctx)
	if err != nil {
		return nil, mcpTagsOutput{}, err
	}
	out := make([]tagDTO, 0, len(records))
	for _, record := range records {
		out = append(out, toTagDTO(record))
	}
	return nil, mcpTagsOutput{Tags: out}, nil
}

func (s *Server) mcpAdminCreateTag(ctx context.Context, req *mcp.CallToolRequest, input mcpCreateTagInput) (*mcp.CallToolResult, mcpTagOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpTagOutput{}, err
	}
	localized := catalogmeta.CleanLocalizedText(input.NameI18n)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = localized.Fallback("")
	}
	if name == "" {
		return nil, mcpTagOutput{}, errors.New("tag name is required")
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	localized = catalogmeta.WithLocalizedDefaults(localized, name)
	record, err := s.db.Tag.Create().
		SetName(name).
		SetNameI18n(catalogmeta.EncodeLocalizedText(localized)).
		SetSlug(slug).
		Save(ctx)
	if err != nil {
		return nil, mcpTagOutput{}, fmt.Errorf("create tag: %w", err)
	}
	s.invalidateSourceFeed()
	return nil, mcpTagOutput{Tag: toTagDTO(record)}, nil
}

func (s *Server) mcpAdminUpdateTag(ctx context.Context, req *mcp.CallToolRequest, input mcpUpdateTagInput) (*mcp.CallToolResult, mcpTagOutput, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, mcpTagOutput{}, err
	}
	if input.ID <= 0 {
		return nil, mcpTagOutput{}, errors.New("tag ID is required")
	}
	update := s.db.Tag.UpdateOneID(input.ID)
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
	record, err := update.Save(ctx)
	if err != nil {
		return nil, mcpTagOutput{}, fmt.Errorf("update tag: %w", err)
	}
	s.invalidateSourceFeed()
	return nil, mcpTagOutput{Tag: toTagDTO(record)}, nil
}

func (s *Server) mcpAdminDeleteTag(ctx context.Context, req *mcp.CallToolRequest, input mcpDeleteTagInput) (*mcp.CallToolResult, okResponse, error) {
	if err := s.requireMCPAdmin(ctx, req); err != nil {
		return nil, okResponse{}, err
	}
	if input.ID <= 0 {
		return nil, okResponse{}, errors.New("tag ID is required")
	}
	if _, err := s.db.Tag.Get(ctx, input.ID); err != nil {
		return nil, okResponse{}, err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, okResponse{}, err
	}
	defer tx.Rollback()
	if _, err := tx.AppTag.Delete().Where(apptag.TagIDEQ(input.ID)).Exec(ctx); err != nil {
		return nil, okResponse{}, err
	}
	if err := tx.Tag.DeleteOneID(input.ID).Exec(ctx); err != nil {
		return nil, okResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return nil, okResponse{}, err
	}
	s.invalidateSourceFeed()
	return nil, okResponse{OK: true}, nil
}
