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
	dw, err := firstparty.GetDefaultWrapper(configPath, client)
	if err != nil {
		return nil, err
	}

	wm := registry.NewWrapperManager(dw)

	fpWrappers, err := firstparty.GetAllWrappers(configPath, client)
	if err != nil {
		return nil, err
	}

	cWrappers, err := contrib.GetAllWrappers(client)
	if err != nil {
		return nil, err
	}

	wm.Add(fpWrappers...)
	wm.Add(cWrappers...)

	return wm, nil
}
