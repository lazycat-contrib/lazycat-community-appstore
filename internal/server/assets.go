package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/asset"
	"lazycat.community/appstore/ent/assetlink"
	"lazycat.community/appstore/internal/assetdata"
	"lazycat.community/appstore/internal/lpkmeta"
)

const (
	serverAssetURLPrefix = "/api/v1/assets"
	maxAssetImageSize    = 2 << 20
	maxLPKIconInputSize  = 16 << 20
	maxLPKIconSide       = 256

	assetOwnerApp  = "app"
	assetOwnerSite = "site"
	assetRoleIcon  = "icon"
)

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found", nil)
		return
	}
	record, err := s.db.Asset.Query().
		Where(asset.IDEQ(id)).
		Select(asset.FieldMediaType, asset.FieldSha256, asset.FieldSize).
		Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found", nil)
		return
	}
	if assetdata.ServeImageMetadata(w, r, record.MediaType, record.Sha256, record.Size) {
		return
	}
	dataRecord, err := s.db.Asset.Query().
		Where(asset.IDEQ(id)).
		Select(asset.FieldData).
		Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found", nil)
		return
	}
	assetdata.ServeImage(w, r, record.MediaType, record.Sha256, dataRecord.Data)
}

func (s *Server) saveAsset(ctx context.Context, payload assetdata.Payload) (*entgo.Asset, error) {
	sum := sha256.Sum256(payload.Data)
	sha := hex.EncodeToString(sum[:])
	if existing, err := s.db.Asset.Query().Where(asset.Sha256EQ(sha)).Only(ctx); err == nil {
		return existing, nil
	} else if !entgo.IsNotFound(err) {
		return nil, err
	}
	record, err := s.db.Asset.Create().
		SetSha256(sha).
		SetMediaType(payload.MediaType).
		SetSize(int64(len(payload.Data))).
		SetData(payload.Data).
		Save(ctx)
	if err == nil {
		return record, nil
	}
	if entgo.IsConstraintError(err) {
		if existing, queryErr := s.db.Asset.Query().Where(asset.Sha256EQ(sha)).Only(ctx); queryErr == nil {
			return existing, nil
		}
	}
	return nil, err
}

func (s *Server) saveLPKIconAsset(ctx context.Context, meta lpkmeta.Metadata) (string, int, error) {
	if len(meta.IconData) == 0 {
		return "", 0, nil
	}
	payload, err := assetdata.NormalizeImage(meta.IconData, meta.IconMediaType, maxLPKIconInputSize, maxLPKIconSide)
	if err != nil {
		return "", 0, fmt.Errorf("invalid LPK icon: %w", err)
	}
	record, err := s.saveAsset(ctx, payload)
	if err != nil {
		return "", 0, err
	}
	return assetdata.URL(serverAssetURLPrefix, record.ID), record.ID, nil
}

func (s *Server) saveAppIconDataURLAsset(ctx context.Context, raw string) (string, int, error) {
	payload, err := assetdata.ParseDataURL(raw, maxLPKIconInputSize)
	if err != nil {
		return "", 0, err
	}
	normalized, err := assetdata.NormalizeImage(payload.Data, payload.MediaType, maxLPKIconInputSize, maxLPKIconSide)
	if err != nil {
		return "", 0, err
	}
	record, err := s.saveAsset(ctx, normalized)
	if err != nil {
		return "", 0, err
	}
	return assetdata.URL(serverAssetURLPrefix, record.ID), record.ID, nil
}

func (s *Server) saveDataURLAsset(ctx context.Context, raw string) (string, int, error) {
	payload, err := assetdata.ParseDataURL(raw, maxAssetImageSize)
	if err != nil {
		return "", 0, err
	}
	record, err := s.saveAsset(ctx, payload)
	if err != nil {
		return "", 0, err
	}
	return assetdata.URL(serverAssetURLPrefix, record.ID), record.ID, nil
}

func (s *Server) materializeAppIconURL(ctx context.Context, input *createAppJSON) error {
	if input == nil || input.iconAssetID > 0 {
		return nil
	}
	iconURL := strings.TrimSpace(input.IconURL)
	if iconURL == "" || !strings.HasPrefix(iconURL, "data:") {
		return nil
	}
	url, assetID, err := s.saveAppIconDataURLAsset(ctx, iconURL)
	if err != nil {
		return fmt.Errorf("invalid app icon: %w", err)
	}
	input.IconURL = url
	input.iconAssetID = assetID
	return nil
}

func (s *Server) materializeSiteIconSetting(ctx context.Context, value string) (string, int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0, nil
	}
	if isDataURL(value) {
		return s.saveDataURLAsset(ctx, value)
	}
	if assetID, ok := s.assetIDFromURL(value); ok {
		return value, assetID, nil
	}
	return value, 0, nil
}

func (s *Server) linkAsset(ctx context.Context, ownerType string, ownerID int, role string, assetID int) error {
	if assetID <= 0 {
		return nil
	}
	_, err := s.db.AssetLink.Create().
		SetAssetID(assetID).
		SetOwnerType(ownerType).
		SetOwnerID(ownerID).
		SetRole(role).
		Save(ctx)
	if err == nil || entgo.IsConstraintError(err) {
		return nil
	}
	return err
}

func (s *Server) replaceAssetLinks(ctx context.Context, ownerType string, ownerID int, role string, assetIDs ...int) error {
	oldLinks, err := s.db.AssetLink.Query().
		Where(assetlink.OwnerTypeEQ(ownerType), assetlink.OwnerIDEQ(ownerID), assetlink.RoleEQ(role)).
		All(ctx)
	if err != nil {
		return err
	}
	oldAssetIDs := make([]int, 0, len(oldLinks))
	for _, link := range oldLinks {
		oldAssetIDs = append(oldAssetIDs, link.AssetID)
	}
	if _, err := s.db.AssetLink.Delete().
		Where(assetlink.OwnerTypeEQ(ownerType), assetlink.OwnerIDEQ(ownerID), assetlink.RoleEQ(role)).
		Exec(ctx); err != nil {
		return err
	}
	seen := map[int]struct{}{}
	for _, assetID := range assetIDs {
		if assetID <= 0 {
			continue
		}
		if _, ok := seen[assetID]; ok {
			continue
		}
		seen[assetID] = struct{}{}
		if err := s.linkAsset(ctx, ownerType, ownerID, role, assetID); err != nil {
			return err
		}
	}
	return s.cleanupAssetIDs(ctx, oldAssetIDs...)
}

func (s *Server) deleteAssetLinksForOwner(ctx context.Context, ownerType string, ownerID int) error {
	links, err := s.db.AssetLink.Query().
		Where(assetlink.OwnerTypeEQ(ownerType), assetlink.OwnerIDEQ(ownerID)).
		All(ctx)
	if err != nil {
		return err
	}
	assetIDs := make([]int, 0, len(links))
	for _, link := range links {
		assetIDs = append(assetIDs, link.AssetID)
	}
	if _, err := s.db.AssetLink.Delete().
		Where(assetlink.OwnerTypeEQ(ownerType), assetlink.OwnerIDEQ(ownerID)).
		Exec(ctx); err != nil {
		return err
	}
	return s.cleanupAssetIDs(ctx, assetIDs...)
}

func (s *Server) cleanupAssetIDs(ctx context.Context, ids ...int) error {
	for _, id := range uniquePositiveInts(ids) {
		linked, err := s.db.AssetLink.Query().Where(assetlink.AssetIDEQ(id)).Exist(ctx)
		if err != nil {
			return err
		}
		if linked {
			continue
		}
		if err := s.db.Asset.DeleteOneID(id).Exec(ctx); err != nil && !entgo.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (s *Server) assetIDFromURL(value string) (int, bool) {
	return assetdata.AssetIDFromURL(serverAssetURLPrefix, value)
}

func uniquePositiveInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isDataURL(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "data:")
}
