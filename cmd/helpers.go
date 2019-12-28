package cmd

import (
	"os"
	"path/filepath"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/spf13/cobra"
)

func getDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		cPath := filepath.ToSlash(
			filepath.Join(homeDir, ".docker", "config.json"),
		)
		if _, err := os.Stat(cPath); err != nil {
			return ""
		}
		return cPath
	}
	return ""
}

func getDefaultWrapperManager(
	cmd *cobra.Command,
	client *registry.HTTPClient,
) (*registry.WrapperManager, error) {
	configPath, err := cmd.Flags().GetString("config-file")
	if err != nil {
		return nil, err
	}
	configPath = filepath.ToSlash(configPath)
	defaultWrapper, err := registry.NewDockerWrapper(configPath, client)
	if err != nil {
		return nil, err
	}
	ACRWrapper, err := registry.NewACRWrapper(configPath, client)
	if err != nil {
		return nil, err
	}
	wrapperManager := registry.NewWrapperManager(defaultWrapper)
	wrappers := []registry.Wrapper{
		registry.NewElasticWrapper(client),
		registry.NewMCRWrapper(client),
		ACRWrapper,
	}
	wrapperManager.Add(wrappers...)
	return wrapperManager, nil
}
