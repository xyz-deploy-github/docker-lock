package cmd

import (
	"os"
	"path/filepath"

	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/registry/contrib"
	"github.com/michaelperel/docker-lock/registry/firstparty"
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
	configPath string,
	client *registry.HTTPClient,
) (*registry.WrapperManager, error) {
	defaultWrapper, err := firstparty.GetDefaultWrapper(configPath, client)
	if err != nil {
		return nil, err
	}

	wrapperManager := registry.NewWrapperManager(defaultWrapper)

	firstPartyWrappers, err := firstparty.GetAllWrappers(configPath, client)
	if err != nil {
		return nil, err
	}

	contribWrappers, err := contrib.GetAllWrappers(client)
	if err != nil {
		return nil, err
	}

	wrapperManager.Add(firstPartyWrappers...)
	wrapperManager.Add(contribWrappers...)

	return wrapperManager, nil
}
