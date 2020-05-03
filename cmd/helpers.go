package cmd

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/michaelperel/docker-lock/registry"
	"github.com/michaelperel/docker-lock/registry/contrib"
	"github.com/michaelperel/docker-lock/registry/firstparty"
)

func defaultConfigPath() string {
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

func defaultWrapperManager(
	configPath string,
	client *registry.HTTPClient,
) (*registry.WrapperManager, error) {
	dw, err := firstparty.DefaultWrapper(configPath, client)
	if err != nil {
		return nil, err
	}

	wm := registry.NewWrapperManager(dw)

	fpWrappers, err := firstparty.AllWrappers(configPath, client)
	if err != nil {
		return nil, err
	}

	cWrappers, err := contrib.AllWrappers(client)
	if err != nil {
		return nil, err
	}

	wm.Add(fpWrappers...)
	wm.Add(cWrappers...)

	return wm, nil
}

func loadEnv(p string) error {
	if err := godotenv.Load(p); err != nil {
		if p != ".env" {
			return err
		}
	}

	return nil
}
