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
	Provider        string `json:"provider"`
	DeliveryMode    string `json:"deliveryMode"`
	LocalPath       string `json:"localPath"`
	EndpointURL     string `json:"endpointUrl"`
	BucketName      string `json:"bucketName"`
	Region          string `json:"region"`
	PathStyle       bool   `json:"pathStyle"`
	AccountID       string `json:"accountId"`
	RootPrefix      string `json:"rootPrefix"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	WebDAVUsername  string `json:"webdavUsername"`
	WebDAVPassword  string `json:"webdavPassword"`
	PublicBaseURL   string `json:"publicBaseUrl"`
}

type appStorageConfig struct {
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

func (s *Server) handleGetStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	cfg, err := s.effectiveStorageConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"storage": s.storageConfigDTO(r.Context(), cfg)})
}

func (s *Server) handleUpdateStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input storageConfigInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	current, err := s.effectiveStorageConfig(r.Context())
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
	writeJSON(w, http.StatusOK, map[string]any{"storage": s.storageConfigDTO(r.Context(), next)})
}

func (s *Server) handleTestStorageConfig(w http.ResponseWriter, r *http.Request, u *entgo.User) {
	var input storageConfigInput
	if err := decodeJSON(r, &input); err != nil {
		badRequest(w, err)
		return
	}
	current, err := s.effectiveStorageConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_CONFIG_FAILED", "Could not load storage config", nil)
		return
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

func (s *Server) handleProxyFile(w http.ResponseWriter, r *http.Request) {
	objectPath := strings.TrimSpace(r.PathValue("path"))
	if objectPath == "" || strings.Contains(objectPath, "..") {
		writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found", nil)
		return
	}
	backend, err := s.storageBackend(r.Context())
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

func (s *Server) deleteStoredObject(ctx context.Context, objectPath string) {
	if strings.TrimSpace(objectPath) == "" {
		return
	}
	backend, err := s.storageBackend(ctx)
	if err == nil {
		_ = backend.Delete(ctx, objectPath)
		return
	}
	_ = s.storage.Delete(ctx, objectPath)
}

func (s *Server) effectiveStorageConfig(ctx context.Context) (appStorageConfig, error) {
	record, err := s.db.StorageConfig.Query().Where(storageconfig.KeyEQ(primaryStorageConfigKey)).Only(ctx)
	if err == nil {
		return appStorageConfigFromRecord(record), nil
	}
	if !entgo.IsNotFound(err) {
		return appStorageConfig{}, err
	}
	return s.storageConfigFromEnv(), nil
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
	if err := validateStorageConfig(cfg); err != nil {
		return err
	}
	record, err := s.db.StorageConfig.Query().Where(storageconfig.KeyEQ(primaryStorageConfigKey)).Only(ctx)
	if err == nil {
		_, err = s.db.StorageConfig.UpdateOneID(record.ID).
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
		SetKey(primaryStorageConfigKey).
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
	return "/api/v1/files/"
}

func (s *Server) storageConfigDTO(ctx context.Context, cfg appStorageConfig) storageConfigDTO {
	return storageConfigDTO{
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
		ServerProxyBaseURL:   strings.TrimRight(s.sitePublicURL(ctx), "/") + "/api/v1/files/",
		EffectiveFileURLMode: cfg.DeliveryMode,
	}
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
