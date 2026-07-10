package clientserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientasset"
	"lazycat.community/appstore/ent/clientassetlink"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/assetdata"
)

const (
	clientAssetURLPrefix      = "/api/client/v1/assets"
	clientAssetOwnerSourceApp = "source_app"
	clientAssetRoleIcon       = "icon"
	clientAssetMaxImageSize   = 2 << 20
	clientAssetFetchTimeout   = 10 * time.Second
)

func (s *Server) handleClientAsset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found")
		return
	}
	record, err := s.db.ClientAsset.Query().
		Where(clientasset.IDEQ(id)).
		Select(clientasset.FieldMediaType, clientasset.FieldSha256, clientasset.FieldSize).
		Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found")
		return
	}
	if assetdata.ServeImageMetadata(w, r, record.MediaType, record.Sha256, record.Size) {
		return
	}
	dataRecord, err := s.db.ClientAsset.Query().
		Where(clientasset.IDEQ(id)).
		Select(clientasset.FieldData).
		Only(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "ASSET_NOT_FOUND", "Asset not found")
		return
	}
	assetdata.ServeImage(w, r, record.MediaType, record.Sha256, dataRecord.Data)
}

func (s *Server) materializeSourceIcon(ctx context.Context, sourceURL, sourcePassword, iconURL string) (string, int, error) {
	iconURL = strings.TrimSpace(iconURL)
	if iconURL == "" {
		return "", 0, nil
	}
	if strings.HasPrefix(iconURL, "data:") {
		payload, err := assetdata.ParseDataURL(iconURL, clientAssetMaxImageSize)
		if err != nil {
			return "", 0, nil
		}
		record, err := s.saveClientAsset(ctx, payload)
		if err != nil {
			return "", 0, err
		}
		return assetdata.URL(clientAssetURLPrefix, record.ID), record.ID, nil
	}
	icon, ok := sameOriginIconURL(sourceURL, iconURL)
	if !ok {
		return iconURL, 0, nil
	}
	iconCtx, cancel := context.WithTimeout(ctx, clientAssetFetchTimeout)
	defer cancel()
	s.ensureHTTPClients()
	payload, err := fetchSourceIcon(iconCtx, noRedirectClient(s.httpClient), icon.String(), sourcePassword, clientAssetMaxImageSize)
	if err != nil {
		return iconURL, 0, nil
	}
	record, err := s.saveClientAsset(ctx, payload)
	if err != nil {
		return "", 0, err
	}
	return assetdata.URL(clientAssetURLPrefix, record.ID), record.ID, nil
}

func (s *Server) saveClientAsset(ctx context.Context, payload assetdata.Payload) (*ent.ClientAsset, error) {
	sum := sha256.Sum256(payload.Data)
	sha := hex.EncodeToString(sum[:])
	if existing, err := s.db.ClientAsset.Query().Where(clientasset.Sha256EQ(sha)).Only(ctx); err == nil {
		return existing, nil
	} else if !ent.IsNotFound(err) {
		return nil, err
	}
	record, err := s.db.ClientAsset.Create().
		SetSha256(sha).
		SetMediaType(payload.MediaType).
		SetSize(int64(len(payload.Data))).
		SetData(payload.Data).
		Save(ctx)
	if err == nil {
		return record, nil
	}
	if ent.IsConstraintError(err) {
		if existing, queryErr := s.db.ClientAsset.Query().Where(clientasset.Sha256EQ(sha)).Only(ctx); queryErr == nil {
			return existing, nil
		}
	}
	return nil, err
}

func (s *Server) linkClientSourceAppIconAssets(ctx context.Context, sourceID int) error {
	records, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.SourceIDEQ(sourceID), clientsourceapp.IconURLHasPrefix(clientAssetURLPrefix+"/")).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		assetID, ok := assetdata.AssetIDFromURL(clientAssetURLPrefix, record.IconURL)
		if !ok {
			continue
		}
		if err := s.linkClientAsset(ctx, clientAssetOwnerSourceApp, record.ID, clientAssetRoleIcon, assetID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) linkClientAsset(ctx context.Context, ownerType string, ownerID int, role string, assetID int) error {
	if assetID <= 0 {
		return nil
	}
	_, err := s.db.ClientAssetLink.Create().
		SetAssetID(assetID).
		SetOwnerType(ownerType).
		SetOwnerID(ownerID).
		SetRole(role).
		Save(ctx)
	if err == nil || ent.IsConstraintError(err) {
		return nil
	}
	return err
}

func (s *Server) deleteClientAssetLinksForOwnerIDs(ctx context.Context, ownerType string, ownerIDs []int) error {
	ownerIDs = uniqueClientPositiveInts(ownerIDs)
	if len(ownerIDs) == 0 {
		return nil
	}
	links, err := s.db.ClientAssetLink.Query().
		Where(clientassetlink.OwnerTypeEQ(ownerType), clientassetlink.OwnerIDIn(ownerIDs...)).
		All(ctx)
	if err != nil {
		return err
	}
	assetIDs := make([]int, 0, len(links))
	for _, link := range links {
		assetIDs = append(assetIDs, link.AssetID)
	}
	if _, err := s.db.ClientAssetLink.Delete().
		Where(clientassetlink.OwnerTypeEQ(ownerType), clientassetlink.OwnerIDIn(ownerIDs...)).
		Exec(ctx); err != nil {
		return err
	}
	return s.cleanupClientAssetIDs(ctx, assetIDs...)
}

func (s *Server) cleanupClientAssetIDs(ctx context.Context, ids ...int) error {
	for _, id := range uniqueClientPositiveInts(ids) {
		linked, err := s.db.ClientAssetLink.Query().Where(clientassetlink.AssetIDEQ(id)).Exist(ctx)
		if err != nil {
			return err
		}
		if linked {
			continue
		}
		if err := s.db.ClientAsset.DeleteOneID(id).Exec(ctx); err != nil && !ent.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func sameOriginIconURL(sourceURL, iconURL string) (*url.URL, bool) {
	base, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, false
	}
	icon, err := url.Parse(strings.TrimSpace(iconURL))
	if err != nil {
		return nil, false
	}
	icon = base.ResolveReference(icon)
	if icon.Scheme != "http" && icon.Scheme != "https" {
		return nil, false
	}
	return icon, strings.EqualFold(icon.Scheme, base.Scheme) && strings.EqualFold(icon.Host, base.Host)
}

func fetchSourceIcon(ctx context.Context, client *http.Client, iconURL, sourcePassword string, maxBytes int64) (assetdata.Payload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return assetdata.Payload{}, err
	}
	if sourcePassword != "" {
		req.Header.Set("X-Source-Password", sourcePassword)
	}
	resp, err := client.Do(req)
	if err != nil {
		return assetdata.Payload{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return assetdata.Payload{}, fmt.Errorf("source icon returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return assetdata.Payload{}, err
	}
	return assetdata.NewImagePayload(raw, resp.Header.Get("Content-Type"), maxBytes)
}

func uniqueClientPositiveInts(values []int) []int {
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
