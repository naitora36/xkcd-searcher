package handler

import (
	"fmt"
	"path/filepath"
	"strings"
)

func getFilePath(rawName string) (string, error) {
	if strings.Contains(rawName, "/") || rawName == "" {
		return "", fmt.Errorf("invalid filename")
	}
	fileName := filepath.Base(rawName)

	if fileName == "." || fileName == "" || strings.Contains(fileName, "..") {
		return "", fmt.Errorf("invalid filename")
	}
	return filepath.Join(nameOfFolder, fileName), nil
}
