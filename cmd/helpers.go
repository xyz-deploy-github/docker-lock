package cmd

import (
	"os"
	"path/filepath"
)

func getDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		cPath := filepath.ToSlash(filepath.Join(homeDir, ".docker", "config.json"))
		if _, err := os.Stat(cPath); err != nil {
			return ""
		}
		return cPath
	}
	return ""
}
