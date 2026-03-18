package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxImageUploadSize = 5 * 1024 * 1024

var allowedImageUploadTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var uploadStemSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func prepareValidatedImageUpload(fileHeader *multipart.FileHeader, entityID uint, uploadDir string) (string, string, error) {
	if fileHeader == nil {
		return "", "", fmt.Errorf("file is required")
	}

	if fileHeader.Size <= 0 {
		return "", "", fmt.Errorf("file is empty")
	}

	if fileHeader.Size > maxImageUploadSize {
		return "", "", fmt.Errorf("file size too large. Maximum 5MB allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", "", fmt.Errorf("failed to open uploaded file")
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", "", fmt.Errorf("failed to inspect uploaded file")
	}

	detectedContentType := http.DetectContentType(header[:n])
	ext, ok := allowedImageUploadTypes[detectedContentType]
	if !ok {
		return "", "", fmt.Errorf("invalid file type. Only JPEG, PNG, GIF, and WebP images are allowed")
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create upload directory")
	}

	filename, err := buildSafeUploadFilename(entityID, fileHeader.Filename, ext)
	if err != nil {
		return "", "", fmt.Errorf("failed to prepare upload filename")
	}

	return filename, filepath.Join(uploadDir, filename), nil
}

func buildSafeUploadFilename(entityID uint, originalFilename, ext string) (string, error) {
	stem := strings.TrimSuffix(filepath.Base(originalFilename), filepath.Ext(originalFilename))
	stem = uploadStemSanitizer.ReplaceAllString(stem, "-")
	stem = strings.Trim(stem, "-_")
	if stem == "" {
		stem = "image"
	}
	if len(stem) > 40 {
		stem = stem[:40]
	}

	randomSuffix := make([]byte, 4)
	if _, err := rand.Read(randomSuffix); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d_%d_%s_%s%s", entityID, time.Now().Unix(), stem, hex.EncodeToString(randomSuffix), ext), nil
}
