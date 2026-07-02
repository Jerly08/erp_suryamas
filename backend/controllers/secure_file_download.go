package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func sendStoredUploadAttachment(c *gin.Context, storedPath, fileName, mimeType string, allowedDirs ...string) {
	filePath, err := resolveStoredUploadPath(storedPath, allowedDirs...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File tidak ditemukan di server"})
		return
	}

	downloadName := strings.TrimSpace(fileName)
	if downloadName == "" {
		downloadName = filepath.Base(filePath)
	}
	if strings.TrimSpace(mimeType) != "" {
		c.Header("Content-Type", mimeType)
	}
	c.FileAttachment(filePath, downloadName)
}

func resolveStoredUploadPath(storedPath string, allowedDirs ...string) (string, error) {
	cleanInput := strings.TrimSpace(storedPath)
	if cleanInput == "" {
		return "", fmt.Errorf("file path is empty")
	}

	cleanInput = strings.TrimPrefix(cleanInput, ".")
	cleanInput = strings.TrimPrefix(cleanInput, "/")
	cleanInput = filepath.Clean(cleanInput)
	if cleanInput == "." || strings.HasPrefix(cleanInput, "..") {
		return "", fmt.Errorf("invalid file path")
	}

	uploadRoot, err := filepath.Abs("uploads")
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload root")
	}
	fullPath, err := filepath.Abs(cleanInput)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path")
	}

	if fullPath == uploadRoot || !strings.HasPrefix(fullPath, uploadRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path is outside upload storage")
	}

	for _, allowedDir := range allowedDirs {
		allowedDir = strings.Trim(strings.TrimSpace(allowedDir), "/")
		if allowedDir == "" {
			continue
		}
		allowedPath, err := filepath.Abs(filepath.Join("uploads", allowedDir))
		if err != nil {
			return "", fmt.Errorf("failed to resolve allowed upload path")
		}
		if fullPath == allowedPath || strings.HasPrefix(fullPath, allowedPath+string(os.PathSeparator)) {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("file type is not available for this download endpoint")
}
