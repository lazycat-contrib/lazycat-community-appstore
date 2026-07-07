package server

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"lazycat.community/appstore/internal/storage"
)

const (
	maxAvatarImageSize   = 2 << 20
	maxSiteIconImageSize = 2 << 20
)

func validateUploadedImage(file multipart.File, header *multipart.FileHeader, maxBytes int64) error {
	if maxBytes <= 0 {
		return errors.New("max image size must be positive")
	}
	if header.Size > maxBytes {
		return storage.ErrTooLarge
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return errors.New("image must be png, jpg, jpeg, or webp")
	}
	var buf [512]byte
	n, err := file.Read(buf[:])
	if err != nil && err != io.EOF {
		return err
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return seekErr
	}
	contentType := http.DetectContentType(buf[:n])
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return nil
	default:
		return errors.New("image content type is not supported")
	}
}
