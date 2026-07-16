package clientserver

import (
	"context"
	"strconv"
	"strings"
	"time"

	"lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/clientsetting"
	"lazycat.community/appstore/ent/clientsourceapp"
	"lazycat.community/appstore/internal/assetdata"
)

const (
	currentClientSchemaVersion = 3
	systemClientUserID         = "_system"
	settingClientSchemaVersion = "schema_version"
)

func migrateSchema(ctx context.Context, db *ent.Client) error {
	server := &Server{db: db}
	version := storedClientSchemaVersion(ctx, db)
	if version >= currentClientSchemaVersion {
		return nil
	}
	if version < 1 {
		if err := server.migrateClientIconDataURLs(ctx); err != nil {
			return err
		}
		if err := setSystemClientSetting(ctx, db, settingClientSchemaVersion, "1"); err != nil {
			return err
		}
	}
	if version < 2 {
		if err := setSystemClientSetting(ctx, db, settingClientSchemaVersion, "2"); err != nil {
			return err
		}
	}
	if version < 3 {
		if err := server.invalidateLegacySourceUpdateTimes(ctx); err != nil {
			return err
		}
		if err := setSystemClientSetting(ctx, db, settingClientSchemaVersion, "3"); err != nil {
			return err
		}
	}
	return nil
}

func storedClientSchemaVersion(ctx context.Context, db *ent.Client) int {
	record, err := db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(systemClientUserID), clientsetting.KeyEQ(settingClientSchemaVersion)).
		Only(ctx)
	if err != nil {
		return 0
	}
	version, err := strconv.Atoi(strings.TrimSpace(record.Value))
	if err != nil || version < 0 {
		return 0
	}
	return version
}

func setSystemClientSetting(ctx context.Context, db *ent.Client, key, value string) error {
	record, err := db.ClientSetting.Query().
		Where(clientsetting.UserIDEQ(systemClientUserID), clientsetting.KeyEQ(key)).
		Only(ctx)
	if err == nil {
		_, err = db.ClientSetting.UpdateOneID(record.ID).SetValue(value).Save(ctx)
		return err
	}
	if !ent.IsNotFound(err) {
		return err
	}
	_, err = db.ClientSetting.Create().SetUserID(systemClientUserID).SetKey(key).SetValue(value).Save(ctx)
	return err
}

func (s *Server) invalidateLegacySourceUpdateTimes(ctx context.Context) error {
	if _, err := s.db.ClientSourceApp.Update().SetUpdatedAt(time.Unix(0, 0).UTC()).Save(ctx); err != nil {
		return err
	}
	_, err := s.db.ClientSource.Update().SetLastEtag("").Save(ctx)
	return err
}

func (s *Server) migrateClientIconDataURLs(ctx context.Context) error {
	records, err := s.db.ClientSourceApp.Query().
		Where(clientsourceapp.IconURLHasPrefix("data:")).
		All(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		payload, err := assetdata.ParseDataURL(record.IconURL, clientAssetMaxImageSize)
		if err != nil {
			continue
		}
		assetRecord, err := s.saveClientAsset(ctx, payload)
		if err != nil {
			return err
		}
		nextURL := assetdata.URL(clientAssetURLPrefix, assetRecord.ID)
		if _, err := s.db.ClientSourceApp.UpdateOneID(record.ID).SetIconURL(nextURL).Save(ctx); err != nil {
			_ = s.cleanupClientAssetIDs(ctx, assetRecord.ID)
			return err
		}
		if err := s.linkClientAsset(ctx, clientAssetOwnerSourceApp, record.ID, clientAssetRoleIcon, assetRecord.ID); err != nil {
			return err
		}
	}
	return nil
}
