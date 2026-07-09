package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	entgo "lazycat.community/appstore/ent"
	"lazycat.community/appstore/ent/storageconfig"
	"lazycat.community/appstore/internal/storage"
)

const (
	primaryStorageConfigKey = "primary"

	storageProviderLocal        = "LOCAL"
	storageProviderS3           = "S3"
	storageProviderCloudflareR2 = "CLOUDFLARE_R2"
	storageProviderWebDAV       = "WEBDAV"

	storageDeliveryServer = "SERVER"
	storageDeliveryDirect = "DIRECT"
)

type storageConfigDTO struct {
	Key                  string `json:"key"`
	Name                 string `json:"name"`
	IsDefault            bool   `json:"isDefault"`
	Provider             string `json:"provider"`
	DeliveryMode         string `json:"deliveryMode"`
	LocalPath            string `json:"localPath"`
	EndpointURL          string `json:"endpointUrl"`
	BucketName           string `json:"bucketName"`
	Region               string `json:"region"`
	PathStyle            bool   `json:"pathStyle"`
	AccountID            string `json:"accountId"`
	RootPrefix           string `json:"rootPrefix"`
	AccessKeyID          string `json:"accessKeyId"`
	SecretAccessKeySet   bool   `json:"secretAccessKeySet"`
	WebDAVUsername       string `json:"webdavUsername"`
	WebDAVPasswordSet    bool   `json:"webdavPasswordSet"`
	PublicBaseURL        string `json:"publicBaseUrl"`
	ServerProxyBaseURL   string `json:"serverProxyBaseUrl"`
	EffectiveFileURLMode string `json:"effectiveFileUrlMode"`
}

type storageConfigInput struct {
	Key                  string `json:"key"`
	Name                 string `json:"name"`
	IsDefault            bool   `json:"isDefault"`
	Provider             string `json:"provider"`
	DeliveryMode         string `json:"deliveryMode"`
	LocalPath            string `json:"localPath"`
	EndpointURL          string `json:"endpointUrl"`
	BucketName           string `json:"bucketName"`
	Region               string `json:"region"`
	PathStyle            bool   `json:"pathStyle"`
	AccountID            string `json:"accountId"`
	RootPrefix           string `json:"rootPrefix"`
	AccessKeyID          string `json:"accessKeyId"`
	SecretAccessKey      string `json:"secretAccessKey"`
	SecretAccessKeySet   bool   `json:"secretAccessKeySet"`
	WebDAVUsername       string `json:"webdavUsername"`
	WebDAVPassword       string `json:"webdavPassword"`
	WebDAVPasswordSet    bool   `json:"webdavPasswordSet"`
	PublicBaseURL        string `json:"publicBaseUrl"`
	ServerProxyBaseURL   string `json:"serverProxyBaseUrl"`
	EffectiveFileURLMode string `json:"effectiveFileUrlMode"`
}

type appStorageConfig struct {
	Key             string
	Name            string
	Provider        string
	DeliveryMode    string
	LocalPath       string
	EndpointURL     string
	BucketName      string
	Region          string
	PathStyle       bool
	AccountID       string
	RootPrefix      string
	AccessKeyID     string
	SecretAccessKey string
	WebDAVUsername  string
	WebDAVPassword  string
	PublicBaseURL   string
}

type storageOptionDTO struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	IsDefault    bool   `json:"isDefault"`
	Provider     string `json:"provider"`
	DeliveryMode string `json:"deliveryMode"`
}

func (s *Server) handleListStorageOptions(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	configs, defaultKey, err := s.listStorageConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
	}
	options := make([]storageOptionDTO, 0, len(configs))
	for _, cfg := range configs {
		options = append(options, storageOptionDTO{
			Key:          cfg.Key,
			Name:         storageDisplayName(cfg),
			IsDefault:    cfg.Key == defaultKey,
			Provider:     cfg.Provider,
			DeliveryMode: cfg.DeliveryMode,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"storages": options, "defaultKey": defaultKey})
}

func (s *Server) handleListStorageConfigs(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	configs, defaultKey, err := s.listStorageConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
	}
	items := make([]storageConfigDTO, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, s.storageConfigDTO(r.Context(), cfg, cfg.Key == defaultKey))
	}
	writeJSON(w, http.StatusOK, map[string]any{"storages": items, "defaultKey": defaultKey})
}

func (s *Server) handleCreateStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.saveStorageConfigFromRequest(w, r, "")
}

func (s *Server) handleUpdateStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.saveStorageConfigFromRequest(w, r, r.PathValue("key"))
}

func (s *Server) handleUpdateDefaultStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	s.saveStorageConfigFromRequest(w, r, primaryStorageConfigKey)
}

func (s *Server) saveStorageConfigFromRequest(w http.ResponseWriter, r *http.Request, pathKey string) {
	var input storageConfigInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	key := normalizeStorageKey(pathKey)
	if key == "" {
		key = normalizeStorageKey(input.Key)
	}
	if key == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "storage key is required", nil)
		return
	}
	input.Key = key
	current, err := s.storageConfigByKeyOrDefault(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
	}
	next, err := s.storageConfigFromInput(current, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if err := s.saveStorageConfig(r.Context(), next); err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_SAVE_FAILED", "Could not save storage config", nil)
		return
	}
	defaultKey := s.defaultStorageKey(r.Context())
	if defaultKey == "" {
		defaultKey = primaryStorageConfigKey
	}
	writeJSON(w, http.StatusOK, map[string]any{"storage": s.storageConfigDTO(r.Context(), next, next.Key == defaultKey)})
}

func (s *Server) handleDeleteStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	key := normalizeStorageKey(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "storage key is required", nil)
		return
	}
	if key == s.defaultStorageKey(r.Context()) {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "default storage cannot be deleted", nil)
		return
	}
	deleted, err := s.db.StorageConfig.Delete().Where(storageconfig.KeyEQ(key)).Exec(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_DELETE_FAILED", "Could not delete storage config", nil)
		return
	}
	if deleted == 0 {
		writeError(w, http.StatusNotFound, "STORAGE_CONFIG_NOT_FOUND", "Storage config not found", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSetDefaultStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	key := normalizeStorageKey(r.PathValue("key"))
	if key == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "storage key is required", nil)
		return
	}
	if _, err := s.effectiveStorageConfigByKey(r.Context(), key); err != nil {
		writeError(w, http.StatusNotFound, "STORAGE_CONFIG_NOT_FOUND", "Storage config not found", nil)
		return
	}
	if err := s.setSetting(r.Context(), settingDefaultStorageKey, key); err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_SAVE_FAILED", "Could not set default storage", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"defaultKey": key})
}

func (s *Server) handleTestStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input storageConfigInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	key := normalizeStorageKey(input.Key)
	if key == "" {
		key = normalizeStorageKey(r.PathValue("key"))
	}
	current, err := s.storageConfigByKeyOrDefault(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
	}
	if key != "" {
		input.Key = key
	}
	next, err := s.storageConfigFromInput(current, input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	backend, err := storageBackendFromConfig(next)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), backend, strings.NewReader("ok"), "storage-test.txt", 1024)
	if err != nil {
		writeError(w, http.StatusBadGateway, "STORAGE_TEST_FAILED", err.Error(), nil)
		return
	}
	_ = backend.Delete(r.Context(), obj.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "downloadUrl": s.absoluteURL(r.Context(), obj.DownloadURL)})
}

func (s *Server) handleTestSavedStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	cfg, err := s.effectiveStorageConfigByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, "STORAGE_CONFIG_NOT_FOUND", "Storage config not found", nil)
		return
	}
	backend, err := storageBackendFromConfig(cfg)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	obj, err := storage.SaveFile(r.Context(), backend, strings.NewReader("ok"), "storage-test.txt", 1024)
	if err != nil {
		writeError(w, http.StatusBadGateway, "STORAGE_TEST_FAILED", err.Error(), nil)
		return
	}
	_ = backend.Delete(r.Context(), obj.Path)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "downloadUrl": s.absoluteURL(r.Context(), obj.DownloadURL)})
}

func (s *Server) handleProxyFile(w http.ResponseWriter, r *http.Request) {
	storageKey := normalizeStorageKey(r.PathValue("storageKey"))
	objectPath := strings.TrimSpace(r.PathValue("path"))
	if storageKey == "" || objectPath == "" || strings.Contains(objectPath, "..") {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	backend, err := s.storageBackendForKey(r.Context(), storageKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage backend", nil)
		return
	}
	reader, err := backend.Open(r.Context(), objectPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	defer reader.Body.Close()
	if reader.ContentType != "" {
		w.Header().Set("Content-Type", reader.ContentType)
	}
	if reader.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(reader.Size, 10))
	}
	if seeker, ok := reader.Body.(io.ReadSeeker); ok {
		http.ServeContent(w, r, reader.Name, reader.ModTime, seeker)
		return
	}
	_, _ = io.Copy(w, reader.Body)
}

func (s *Server) storageBackend(ctx context.Context) (storage.Backend, error) {
	cfg, err := s.effectiveStorageConfig(ctx)
	if err != nil {
		return nil, err
	}
	return storageBackendFromConfig(cfg)
}

func (s *Server) storageBackendForKey(ctx context.Context, key string) (storage.Backend, error) {
	cfg, err := s.effectiveStorageConfigByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return storageBackendFromConfig(cfg)
}

func (s *Server) deleteStoredObject(ctx context.Context, storageKey, objectPath string) {
	if strings.TrimSpace(objectPath) == "" {
		return
	}
	backend, err := s.storageBackendForKey(ctx, storageKey)
	if err == nil {
		_ = backend.Delete(ctx, objectPath)
		return
	}
	_ = s.storage.Delete(ctx, objectPath)
}

func (s *Server) effectiveStorageConfig(ctx context.Context) (appStorageConfig, error) {
	return s.effectiveStorageConfigByKey(ctx, s.defaultStorageKey(ctx))
}

func (s *Server) effectiveStorageConfigByKey(ctx context.Context, key string) (appStorageConfig, error) {
	key = normalizeStorageKey(key)
	if key == "" {
		key = primaryStorageConfigKey
	}
	record, err := s.db.StorageConfig.Query().Where(storageconfig.KeyEQ(key)).Only(ctx)
	if err == nil {
		return appStorageConfigFromRecord(record), nil
	}
	if !entgo.IsNotFound(err) {
		return appStorageConfig{}, err
	}
	if key == primaryStorageConfigKey {
		cfg := s.storageConfigFromEnv()
		cfg.Key = primaryStorageConfigKey
		cfg.Name = "Primary"
		return cfg, nil
	}
	return appStorageConfig{}, err
}

func (s *Server) storageConfigByKeyOrDefault(ctx context.Context, key string) (appStorageConfig, error) {
	key = normalizeStorageKey(key)
	if key == "" {
		return s.effectiveStorageConfig(ctx)
	}
	cfg, err := s.effectiveStorageConfigByKey(ctx, key)
	if err == nil {
		return cfg, nil
	}
	if !entgo.IsNotFound(err) {
		return appStorageConfig{}, err
	}
	current := s.storageConfigFromEnv()
	current.Key = key
	current.Name = key
	return current, nil
}

func (s *Server) listStorageConfigs(ctx context.Context) ([]appStorageConfig, string, error) {
	records, err := s.db.StorageConfig.Query().Order(entgo.Asc(storageconfig.FieldKey)).All(ctx)
	if err != nil {
		return nil, "", err
	}
	configs := make([]appStorageConfig, 0, len(records)+1)
	for _, record := range records {
		configs = append(configs, appStorageConfigFromRecord(record))
	}
	if len(configs) == 0 {
		cfg := s.storageConfigFromEnv()
		cfg.Key = primaryStorageConfigKey
		cfg.Name = "Primary"
		configs = append(configs, cfg)
	}
	defaultKey := s.defaultStorageKey(ctx)
	hasDefault := false
	for _, cfg := range configs {
		if cfg.Key == defaultKey {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		defaultKey = configs[0].Key
	}
	return configs, defaultKey, nil
}

func (s *Server) defaultStorageKey(ctx context.Context) string {
	key := normalizeStorageKey(s.setting(ctx, settingDefaultStorageKey, primaryStorageConfigKey))
	if key == "" {
		return primaryStorageConfigKey
	}
	return key
}

func (s *Server) uploadStorageKey(ctx context.Context, raw string) (string, error) {
	key := normalizeStorageKey(raw)
	if strings.TrimSpace(raw) != "" && key == "" {
		return "", fmt.Errorf("storageKey is invalid")
	}
	if key == "" {
		key = s.defaultStorageKey(ctx)
	}
	if key == "" {
		key = primaryStorageConfigKey
	}
	if _, err := s.effectiveStorageConfigByKey(ctx, key); err != nil {
		return "", fmt.Errorf("storage %q is not configured", key)
	}
	return key, nil
}

func (s *Server) storageConfigFromEnv() appStorageConfig {
	provider := storageProviderLocal
	switch strings.ToLower(strings.TrimSpace(s.cfg.StorageBackend)) {
	case "s3":
		provider = storageProviderS3
	case "webdav":
		provider = storageProviderWebDAV
	case "", "local":
		provider = storageProviderLocal
	}
	deliveryMode := storageDeliveryServer
	publicBaseURL := ""
	switch provider {
	case storageProviderS3:
		publicBaseURL = cleanURLSetting(s.cfg.S3PublicURL)
	case storageProviderWebDAV:
		publicBaseURL = cleanURLSetting(s.cfg.WebDAVPublicURL)
	}
	if publicBaseURL != "" {
		deliveryMode = storageDeliveryDirect
	}
	return appStorageConfig{
		Key:             primaryStorageConfigKey,
		Name:            "Primary",
		Provider:        provider,
		DeliveryMode:    deliveryMode,
		LocalPath:       strings.TrimSpace(s.cfg.LocalStoragePath),
		EndpointURL:     strings.TrimSpace(s.cfg.S3Endpoint),
		BucketName:      strings.TrimSpace(s.cfg.S3Bucket),
		Region:          "auto",
		PathStyle:       true,
		AccessKeyID:     strings.TrimSpace(s.cfg.S3AccessKey),
		SecretAccessKey: strings.TrimSpace(s.cfg.S3SecretKey),
		WebDAVUsername:  strings.TrimSpace(s.cfg.WebDAVUser),
		WebDAVPassword:  strings.TrimSpace(s.cfg.WebDAVPass),
		PublicBaseURL:   publicBaseURL,
	}
}

func appStorageConfigFromRecord(record *entgo.StorageConfig) appStorageConfig {
	return appStorageConfig{
		Key:             normalizeStorageKey(record.Key),
		Name:            strings.TrimSpace(record.Name),
		Provider:        string(record.Provider),
		DeliveryMode:    string(record.DeliveryMode),
		LocalPath:       strings.TrimSpace(record.LocalPath),
		EndpointURL:     strings.TrimSpace(record.EndpointURL),
		BucketName:      strings.TrimSpace(record.BucketName),
		Region:          strings.TrimSpace(record.Region),
		PathStyle:       record.PathStyle,
		AccountID:       strings.TrimSpace(record.AccountID),
		RootPrefix:      cleanStorageRootPrefix(record.RootPrefix),
		AccessKeyID:     strings.TrimSpace(record.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(record.SecretAccessKey),
		WebDAVUsername:  strings.TrimSpace(record.WebdavUsername),
		WebDAVPassword:  strings.TrimSpace(record.WebdavPassword),
		PublicBaseURL:   cleanURLSetting(record.PublicBaseURL),
	}
}

func (s *Server) storageConfigFromInput(current appStorageConfig, input storageConfigInput) (appStorageConfig, error) {
	next := appStorageConfig{
		Key:             normalizeStorageKey(input.Key),
		Name:            strings.TrimSpace(input.Name),
		Provider:        normalizeStorageProvider(input.Provider),
		DeliveryMode:    normalizeStorageDelivery(input.DeliveryMode),
		LocalPath:       strings.TrimSpace(input.LocalPath),
		EndpointURL:     cleanURLSetting(input.EndpointURL),
		BucketName:      strings.TrimSpace(input.BucketName),
		Region:          strings.TrimSpace(input.Region),
		PathStyle:       input.PathStyle,
		AccountID:       strings.TrimSpace(input.AccountID),
		RootPrefix:      cleanStorageRootPrefix(input.RootPrefix),
		AccessKeyID:     strings.TrimSpace(input.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(input.SecretAccessKey),
		WebDAVUsername:  strings.TrimSpace(input.WebDAVUsername),
		WebDAVPassword:  strings.TrimSpace(input.WebDAVPassword),
		PublicBaseURL:   cleanURLSetting(input.PublicBaseURL),
	}
	if next.Key == "" {
		next.Key = current.Key
	}
	if next.Key == "" {
		next.Key = primaryStorageConfigKey
	}
	if next.Name == "" {
		next.Name = current.Name
	}
	if len([]rune(next.Name)) > 80 {
		return appStorageConfig{}, fmt.Errorf("name must be 80 characters or fewer")
	}
	if next.Provider == "" {
		next.Provider = current.Provider
	}
	if next.DeliveryMode == "" {
		next.DeliveryMode = current.DeliveryMode
	}
	if next.Provider == "" {
		next.Provider = storageProviderLocal
	}
	if next.DeliveryMode == "" {
		next.DeliveryMode = storageDeliveryServer
	}
	if next.Region == "" {
		next.Region = "auto"
	}
	if next.LocalPath == "" {
		next.LocalPath = current.LocalPath
	}
	if next.LocalPath == "" {
		next.LocalPath = s.cfg.LocalStoragePath
	}
	if next.SecretAccessKey == "" {
		next.SecretAccessKey = current.SecretAccessKey
	}
	if next.WebDAVPassword == "" {
		next.WebDAVPassword = current.WebDAVPassword
	}
	if err := validateStorageConfig(next); err != nil {
		return appStorageConfig{}, err
	}
	return next, nil
}

func validateStorageConfig(cfg appStorageConfig) error {
	if normalizeStorageKey(cfg.Key) == "" {
		return fmt.Errorf("key must use letters, numbers, hyphen, underscore, or dot")
	}
	switch cfg.Provider {
	case storageProviderLocal:
		if strings.TrimSpace(cfg.LocalPath) == "" {
			return fmt.Errorf("localPath is required")
		}
	case storageProviderS3, storageProviderCloudflareR2:
		if strings.TrimSpace(cfg.EndpointURL) == "" {
			return fmt.Errorf("endpointUrl is required")
		}
		if !isHTTPURLOrEmpty(cfg.EndpointURL) {
			return fmt.Errorf("endpointUrl must be an http or https URL")
		}
		if strings.TrimSpace(cfg.BucketName) == "" {
			return fmt.Errorf("bucketName is required")
		}
		if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
			return fmt.Errorf("S3 credentials are required")
		}
	case storageProviderWebDAV:
		if strings.TrimSpace(cfg.EndpointURL) == "" {
			return fmt.Errorf("endpointUrl is required")
		}
		if !isHTTPURLOrEmpty(cfg.EndpointURL) {
			return fmt.Errorf("endpointUrl must be an http or https URL")
		}
	default:
		return fmt.Errorf("provider must be LOCAL, S3, CLOUDFLARE_R2, or WEBDAV")
	}
	switch cfg.DeliveryMode {
	case storageDeliveryServer:
	case storageDeliveryDirect:
		if strings.TrimSpace(cfg.PublicBaseURL) == "" {
			return fmt.Errorf("publicBaseUrl is required when deliveryMode is DIRECT")
		}
		if !isHTTPURLOrEmpty(cfg.PublicBaseURL) {
			return fmt.Errorf("publicBaseUrl must be an http or https URL")
		}
	default:
		return fmt.Errorf("deliveryMode must be SERVER or DIRECT")
	}
	return nil
}

func (s *Server) saveStorageConfig(ctx context.Context, cfg appStorageConfig) error {
	cfg.Key = normalizeStorageKey(cfg.Key)
	if err := validateStorageConfig(cfg); err != nil {
		return err
	}
	record, err := s.db.StorageConfig.Query().Where(storageconfig.KeyEQ(cfg.Key)).Only(ctx)
	if err == nil {
		_, err = s.db.StorageConfig.UpdateOneID(record.ID).
			SetName(cfg.Name).
			SetProvider(storageconfig.Provider(cfg.Provider)).
			SetDeliveryMode(storageconfig.DeliveryMode(cfg.DeliveryMode)).
			SetLocalPath(cfg.LocalPath).
			SetEndpointURL(cfg.EndpointURL).
			SetBucketName(cfg.BucketName).
			SetRegion(cfg.Region).
			SetPathStyle(cfg.PathStyle).
			SetAccountID(cfg.AccountID).
			SetRootPrefix(cfg.RootPrefix).
			SetAccessKeyID(cfg.AccessKeyID).
			SetSecretAccessKey(cfg.SecretAccessKey).
			SetWebdavUsername(cfg.WebDAVUsername).
			SetWebdavPassword(cfg.WebDAVPassword).
			SetPublicBaseURL(cfg.PublicBaseURL).
			Save(ctx)
		return err
	}
	if !entgo.IsNotFound(err) {
		return err
	}
	_, err = s.db.StorageConfig.Create().
		SetKey(cfg.Key).
		SetName(cfg.Name).
		SetProvider(storageconfig.Provider(cfg.Provider)).
		SetDeliveryMode(storageconfig.DeliveryMode(cfg.DeliveryMode)).
		SetLocalPath(cfg.LocalPath).
		SetEndpointURL(cfg.EndpointURL).
		SetBucketName(cfg.BucketName).
		SetRegion(cfg.Region).
		SetPathStyle(cfg.PathStyle).
		SetAccountID(cfg.AccountID).
		SetRootPrefix(cfg.RootPrefix).
		SetAccessKeyID(cfg.AccessKeyID).
		SetSecretAccessKey(cfg.SecretAccessKey).
		SetWebdavUsername(cfg.WebDAVUsername).
		SetWebdavPassword(cfg.WebDAVPassword).
		SetPublicBaseURL(cfg.PublicBaseURL).
		Save(ctx)
	return err
}

func storageBackendFromConfig(cfg appStorageConfig) (storage.Backend, error) {
	if err := validateStorageConfig(cfg); err != nil {
		return nil, err
	}
	publicPrefix := storagePublicPrefix(cfg)
	switch cfg.Provider {
	case storageProviderLocal:
		return storage.NewLocalBackend(cfg.LocalPath, publicPrefix), nil
	case storageProviderS3, storageProviderCloudflareR2:
		return storage.NewS3Backend(storage.S3Options{
			Endpoint:   cfg.EndpointURL,
			Bucket:     cfg.BucketName,
			Region:     cfg.Region,
			AccessKey:  cfg.AccessKeyID,
			SecretKey:  cfg.SecretAccessKey,
			UseSSL:     true,
			PathStyle:  cfg.PathStyle,
			RootPrefix: cfg.RootPrefix,
			PublicURL:  publicPrefix,
		})
	case storageProviderWebDAV:
		return storage.NewWebDAVBackend(cfg.EndpointURL, cfg.WebDAVUsername, cfg.WebDAVPassword, publicPrefix, cfg.RootPrefix), nil
	default:
		return nil, fmt.Errorf("unsupported storage provider %q", cfg.Provider)
	}
}

func storagePublicPrefix(cfg appStorageConfig) string {
	if cfg.DeliveryMode == storageDeliveryDirect {
		return cleanURLSetting(cfg.PublicBaseURL)
	}
	key := normalizeStorageKey(cfg.Key)
	if key == "" {
		key = primaryStorageConfigKey
	}
	return "/api/v1/files/" + key + "/"
}

func (s *Server) storageConfigDTO(ctx context.Context, cfg appStorageConfig, isDefault bool) storageConfigDTO {
	return storageConfigDTO{
		Key:                  cfg.Key,
		Name:                 storageDisplayName(cfg),
		IsDefault:            isDefault,
		Provider:             cfg.Provider,
		DeliveryMode:         cfg.DeliveryMode,
		LocalPath:            cfg.LocalPath,
		EndpointURL:          cfg.EndpointURL,
		BucketName:           cfg.BucketName,
		Region:               cfg.Region,
		PathStyle:            cfg.PathStyle,
		AccountID:            cfg.AccountID,
		RootPrefix:           cfg.RootPrefix,
		AccessKeyID:          cfg.AccessKeyID,
		SecretAccessKeySet:   cfg.SecretAccessKey != "",
		WebDAVUsername:       cfg.WebDAVUsername,
		WebDAVPasswordSet:    cfg.WebDAVPassword != "",
		PublicBaseURL:        cfg.PublicBaseURL,
		ServerProxyBaseURL:   strings.TrimRight(s.sitePublicURL(ctx), "/") + storagePublicPrefix(cfg),
		EffectiveFileURLMode: cfg.DeliveryMode,
	}
}

func storageDisplayName(cfg appStorageConfig) string {
	if strings.TrimSpace(cfg.Name) != "" {
		return strings.TrimSpace(cfg.Name)
	}
	if cfg.Key == primaryStorageConfigKey {
		return "Primary"
	}
	return cfg.Key
}

func normalizeStorageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return ""
	}
	return value
}

func normalizeStorageProvider(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case storageProviderLocal, storageProviderS3, storageProviderCloudflareR2, storageProviderWebDAV:
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeStorageDelivery(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case storageDeliveryServer, storageDeliveryDirect:
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func cleanStorageRootPrefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "." {
		return ""
	}
	return prefix
}
