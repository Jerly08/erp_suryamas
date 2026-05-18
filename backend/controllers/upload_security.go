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
const maxDocumentUploadSize = 20 * 1024 * 1024

var allowedImageUploadTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var uploadStemSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

var allowedDocumentUploadTypes = map[string]map[string]struct{}{
	".pdf": {
		"application/pdf": {},
	},
	".doc": {
		"application/msword":       {},
		"application/octet-stream": {},
	},
	".docx": {
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
		"application/zip":          {},
		"application/octet-stream": {},
	},
	".xls": {
		"application/vnd.ms-excel": {},
		"application/octet-stream": {},
	},
	".xlsx": {
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
		"application/zip":          {},
		"application/octet-stream": {},
	},
	".csv": {
		"text/csv":                  {},
		"application/csv":           {},
		"text/plain; charset=utf-8": {},
		"text/plain":                {},
		"application/octet-stream":  {},
	},
	".txt": {
		"text/plain; charset=utf-8": {},
		"text/plain":                {},
	},
	".jpg": {
		"image/jpeg": {},
	},
	".jpeg": {
		"image/jpeg": {},
	},
	".png": {
		"image/png": {},
	},
	".webp": {
		"image/webp": {},
	},
}

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

func prepareValidatedDocumentUpload(fileHeader *multipart.FileHeader, entityID uint, uploadDir string) (string, string, string, error) {
	if fileHeader == nil {
		return "", "", "", fmt.Errorf("file is required")
	}

	if fileHeader.Size <= 0 {
		return "", "", "", fmt.Errorf("file is empty")
	}

	if fileHeader.Size > maxDocumentUploadSize {
		return "", "", "", fmt.Errorf("file size too large. Maximum 20MB allowed")
	}

	originalExt := strings.ToLower(filepath.Ext(fileHeader.Filename))
	allowedMIMETypes, ok := allowedDocumentUploadTypes[originalExt]
	if !ok {
		return "", "", "", fmt.Errorf("invalid file type. Only PDF, Word, Excel, CSV, TXT, JPG, PNG, and WebP documents are allowed")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open uploaded file")
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", "", "", fmt.Errorf("failed to inspect uploaded file")
	}

	detectedContentType := http.DetectContentType(header[:n])
	if _, ok := allowedMIMETypes[detectedContentType]; !ok {
		return "", "", "", fmt.Errorf("file content does not match the allowed document type")
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create upload directory")
	}

	filename, err := buildSafeUploadFilename(entityID, fileHeader.Filename, originalExt)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to prepare upload filename")
	}

	return filename, filepath.Join(uploadDir, filename), detectedContentType, nil
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
