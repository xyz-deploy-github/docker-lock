package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/safe-waters/docker-lock/registry"
	"github.com/safe-waters/docker-lock/registry/contrib"
	"github.com/safe-waters/docker-lock/registry/firstparty"
)

// defaultConfigPath returns the default location of docker's config.json
// for all platforms.
func defaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		cPath := filepath.Join(homeDir, ".docker", "config.json")
		if _, err := os.Stat(cPath); err != nil {
			return ""
		}

		return cPath
	}

	return ""
}

// defaultWrapperManager returns a wrapper manager where the default
// wrapper is the first party's default wrapper (a wrapper for Docker Hub),
// and all other first party and contrib wrappers are added.
func defaultWrapperManager(
	client *registry.HTTPClient,
	configPath string,
) (*registry.WrapperManager, error) {
	dw, err := firstparty.DefaultWrapper(client, configPath)
	if err != nil {
		return nil, err
	}

	wm := registry.NewWrapperManager(dw)
	wm.Add(firstparty.AllWrappers(client, configPath)...)
	wm.Add(contrib.AllWrappers(client, configPath)...)

	return wm, nil
}

// loadEnv loads environment variables from a dot env file into the process.
// If the default path, ".env", is supplied and does not exist, no error
// occurs. If any other path is supplied and does not exist, an error occurs.
// If the dot env file exists, but cannot be parsed, an error occurs.
func loadEnv(p string) error {
	if err := godotenv.Load(p); err != nil {
		if p != ".env" {
			return err
		}
	}

	return nil
}

// configureLogger configures a common logger for all subcommands. If
// verbose is not set, all logs are discarded.
func configureLogger(verbose bool) {
	if !verbose {
		log.SetOutput(ioutil.Discard)
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix("[DEBUG] ")
}
