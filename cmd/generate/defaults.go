package generate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/safe-waters/docker-lock/generate"
	"github.com/safe-waters/docker-lock/generate/collect"
	"github.com/safe-waters/docker-lock/generate/parse"
	"github.com/safe-waters/docker-lock/generate/update"
	"github.com/safe-waters/docker-lock/registry"
	"github.com/safe-waters/docker-lock/registry/contrib"
	"github.com/safe-waters/docker-lock/registry/firstparty"
)

// DefaultPathCollector creates a PathCollector for Generator.
func DefaultPathCollector(flags *Flags) (generate.IPathCollector, error) {
	if flags == nil {
		return nil, errors.New("flags cannot be nil")
	}

	if flags.DockerfileFlags == nil {
		return nil, errors.New("flags.DockerfileFlags cannot be nil")
	}

	if flags.ComposefileFlags == nil {
		return nil, errors.New("flags.ComposefileFlags cannot be nil")
	}

	var dockerfileCollector *collect.PathCollector

	var composefileCollector *collect.PathCollector

	var err error

	if !flags.DockerfileFlags.ExcludePaths {
		dockerfileCollector, err = collect.NewPathCollector(
			flags.FlagsWithSharedValues.BaseDir, []string{"Dockerfile"},
			flags.DockerfileFlags.ManualPaths, flags.DockerfileFlags.Globs,
			flags.DockerfileFlags.Recursive,
		)
		if err != nil {
			return nil, err
		}
	}

	if !flags.ComposefileFlags.ExcludePaths {
		composefileCollector, err = collect.NewPathCollector(
			flags.FlagsWithSharedValues.BaseDir,
			[]string{"docker-compose.yml", "docker-compose.yaml"},
			flags.ComposefileFlags.ManualPaths, flags.ComposefileFlags.Globs,
			flags.ComposefileFlags.Recursive,
		)
		if err != nil {
			return nil, err
		}
	}

	return &generate.PathCollector{
		DockerfileCollector:  dockerfileCollector,
		ComposefileCollector: composefileCollector,
	}, nil
}

// DefaultImageParser creates an ImageParser for Generator.
func DefaultImageParser(flags *Flags) (generate.IImageParser, error) {
	if flags == nil {
		return nil, errors.New("flags cannot be nil")
	}

	if flags.DockerfileFlags == nil {
		return nil, errors.New("flags.DockerfileFlags cannot be nil")
	}

	if flags.ComposefileFlags == nil {
		return nil, errors.New("flags.ComposefileFlags cannot be nil")
	}

	var dockerfileImageParser *parse.DockerfileImageParser

	var composefileImageParser *parse.ComposefileImageParser

	if !flags.DockerfileFlags.ExcludePaths ||
		!flags.ComposefileFlags.ExcludePaths {
		dockerfileImageParser = &parse.DockerfileImageParser{}
	}

	if !flags.ComposefileFlags.ExcludePaths {
		composefileImageParser = &parse.ComposefileImageParser{}
	}

	return &generate.ImageParser{
		DockerfileImageParser:  dockerfileImageParser,
		ComposefileImageParser: composefileImageParser,
	}, nil
}

// DefaultImageDigestUpdater creates an ImageDigestUpdater for Generator.
func DefaultImageDigestUpdater(
	client *registry.HTTPClient,
	flags *Flags,
) (generate.IImageDigestUpdater, error) {
	if flags == nil {
		return nil, errors.New("flags cannot be nil")
	}

	if flags.DockerfileFlags == nil {
		return nil, errors.New("flags.DockerfileFlags cannot be nil")
	}

	if flags.ComposefileFlags == nil {
		return nil, errors.New("flags.ComposefileFlags cannot be nil")
	}

	var dockerfileImageDigestUpdater *update.DockerfileImageDigestUpdater

	var composefileImageDigestUpdater *update.ComposefileImageDigestUpdater

	var wrapperManager *registry.WrapperManager

	var queryExecutor *update.QueryExecutor

	var err error

	if !flags.DockerfileFlags.ExcludePaths ||
		!flags.ComposefileFlags.ExcludePaths {
		wrapperManager, err = DefaultWrapperManager(
			client, flags.FlagsWithSharedValues.ConfigPath,
		)
		if err != nil {
			return nil, err
		}

		queryExecutor, err = update.NewQueryExecutor(wrapperManager)
		if err != nil {
			return nil, err
		}
	}

	if !flags.DockerfileFlags.ExcludePaths {
		dockerfileImageDigestUpdater = &update.DockerfileImageDigestUpdater{
			QueryExecutor: queryExecutor,
		}
	}

	if !flags.ComposefileFlags.ExcludePaths {
		composefileImageDigestUpdater = &update.ComposefileImageDigestUpdater{
			QueryExecutor: queryExecutor,
		}
	}

	return &generate.ImageDigestUpdater{
		DockerImageUpdater:  dockerfileImageDigestUpdater,
		ComposeImageUpdater: composefileImageDigestUpdater,
	}, nil
}

// DefaultConfigPath returns the default location of docker's config.json
// for all platforms.
func DefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		cPath := filepath.Join(homeDir, ".docker", "config.json")
		if _, err := os.Stat(cPath); err != nil {
			return ""
		}

		return cPath
	}

	return ""
}

// DefaultWrapperManager creates a WrapperManager with all possible Wrappers,
// the default being the docker wrapper.
func DefaultWrapperManager(
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

// DefaultLoadEnv loads .env files based on the path. If a path does not
// exist and that path is not ".env", an error will occur.
func DefaultLoadEnv(path string) error {
	if _, err := os.Stat(path); err != nil {
		if path == ".env" {
			return nil
		}

		return err
	}

	return godotenv.Load(path)
}
