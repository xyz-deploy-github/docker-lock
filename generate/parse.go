package generate

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

// BaseImage represents an image in a Dockerfile's FROM instruction
// or a docker-compose file's image key.
type BaseImage struct {
	Image           *Image
	dockerfilePath  string
	composefilePath string
	position        int
	serviceName     string
	err             error
}

// parseFiles finds all base images in the generator's Dockerfiles and
// docker-compose files.
func (g *Generator) parseFiles(doneCh <-chan struct{}) chan *BaseImage {
	bImCh := make(chan *BaseImage)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go g.parseDfiles(bImCh, &wg, doneCh)

	wg.Add(1)

	go g.parseCfiles(bImCh, &wg, doneCh)

	go func() {
		wg.Wait()
		close(bImCh)
	}()

	return bImCh
}

// parseDfiles finds base images in Dockerfiles.
func (g *Generator) parseDfiles(
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	bArgs := g.dBuildArgs()

	for _, dPath := range g.DockerfilePaths {
		wg.Add(1)

		go g.parseDfile(dPath, bArgs, "", "", bImCh, wg, doneCh)
	}
}

// parseDfile finds base images in a Dockerfile.
func (g *Generator) parseDfile(
	dPath string,
	bArgs map[string]string,
	cPath string,
	svcName string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	dfile, err := os.Open(dPath) // nolint: gosec
	if err != nil {
		g.addErrToBiCh(err, bImCh, doneCh)
		return
	}
	defer dfile.Close()

	pos := 0                     // order of image line in Dockerfile
	stages := map[string]bool{}  // FROM <image line> as <stage>
	gArgs := map[string]string{} // global ARGs before the first FROM
	gCtx := true                 // true if before first FROM
	scnr := bufio.NewScanner(dfile)
	scnr.Split(bufio.ScanLines)

	for scnr.Scan() {
		fields := strings.Fields(scnr.Text())

		const instIndex = 0

		const imLineIndex = 1

		if len(fields) > 0 {
			switch strings.ToLower(fields[instIndex]) {
			case "arg":
				if gCtx {
					if strings.Contains(fields[imLineIndex], "=") {
						//ARG VAR=VAL
						vv := strings.SplitN(fields[imLineIndex], "=", 2)

						const varIndex = 0

						const valIndex = 1

						strpVar := g.stripQuotes(vv[varIndex])
						strpVal := g.stripQuotes(vv[valIndex])
						gArgs[strpVar] = strpVal
					} else {
						// ARG VAR1
						strpVar := g.stripQuotes(fields[imLineIndex])
						gArgs[strpVar] = ""
					}
				}
			case "from":
				gCtx = false
				line := expandField(fields[imLineIndex], gArgs, bArgs)
				im := g.convertImageLineToImage(line)

				if !stages[line] {
					select {
					case <-doneCh:
						return
					case bImCh <- &BaseImage{
						Image:           im,
						dockerfilePath:  dPath,
						composefilePath: cPath,
						serviceName:     svcName,
						position:        pos,
					}:
						pos++
					}
				}
				// FROM <image> AS <stage>
				// FROM <stage> AS <another stage>
				const maxNumFields = 4
				if len(fields) == maxNumFields {
					const stageIndex = 3
					stage := expandField(fields[stageIndex], gArgs, bArgs)
					stages[stage] = true
				}
			}
		}
	}
}

// parseCfiles finds base images in docker-compose files.
func (g *Generator) parseCfiles(
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	for _, cPath := range g.ComposefilePaths {
		wg.Add(1)

		go g.parseCfile(cPath, bImCh, wg, doneCh)
	}
}

// parseCfile finds base images in a docker-compose file.
func (g *Generator) parseCfile(
	cPath string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	ymlByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		g.addErrToBiCh(err, bImCh, doneCh)
		return
	}

	comp := compose{}
	if err := yaml.Unmarshal(ymlByt, &comp); err != nil {
		err = fmt.Errorf("from '%s': %v", cPath, err)
		g.addErrToBiCh(err, bImCh, doneCh)

		return
	}

	for svcName, svc := range comp.Services {
		wg.Add(1)

		go g.parseService(svcName, svc, cPath, bImCh, wg, doneCh)
	}
}

// parseService finds base images in a service.
func (g *Generator) parseService(
	svcName string,
	svc *service,
	cPath string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	if svc.BuildWrapper == nil {
		imLine := os.ExpandEnv(svc.ImageName)
		im := g.convertImageLineToImage(imLine)
		select {
		case <-doneCh:
		case bImCh <- &BaseImage{
			Image:           im,
			composefilePath: cPath,
			serviceName:     svcName,
		}:
		}

		return
	}

	switch build := svc.BuildWrapper.Build.(type) {
	case simple:
		ctx := filepath.ToSlash(os.ExpandEnv(string(build)))
		if !filepath.IsAbs(ctx) {
			ctx = filepath.ToSlash(filepath.Join(filepath.Dir(cPath), ctx))
		}

		dPath := filepath.ToSlash(filepath.Join(ctx, "Dockerfile"))

		wg.Add(1)

		go g.parseDfile(dPath, nil, cPath, svcName, bImCh, wg, doneCh)
	case verbose:
		ctx := filepath.ToSlash(os.ExpandEnv(build.Context))
		if !filepath.IsAbs(ctx) {
			ctx = filepath.ToSlash(filepath.Join(filepath.Dir(cPath), ctx))
		}

		dPath := filepath.ToSlash(os.ExpandEnv(build.DockerfilePath))
		if dPath == "" {
			dPath = "Dockerfile"
		}

		dPath = filepath.ToSlash(filepath.Join(ctx, dPath))

		bArgs := g.cBuildArgs(build)

		wg.Add(1)

		go g.parseDfile(dPath, bArgs, cPath, svcName, bImCh, wg, doneCh)
	}
}

// convertLineToIm converts a line such as ubuntu:bionic into an Image.
func (g *Generator) convertImageLineToImage(line string) *Image {
	tagSeparator := -1
	digestSeparator := -1

loop:
	for i, c := range line {
		switch c {
		case ':':
			tagSeparator = i
		case '@':
			digestSeparator = i
			break loop
		}
	}

	var name, tag, digest string

	// 4 valid cases
	switch {
	case tagSeparator != -1 && digestSeparator != -1:
		// ubuntu:18.04@sha256:9b1702...
		name = line[:tagSeparator]
		tag = line[tagSeparator+1 : digestSeparator]
		digest = line[digestSeparator+1+len("sha256:"):]
	case tagSeparator != -1 && digestSeparator == -1:
		// ubuntu:18.04
		name = line[:tagSeparator]
		tag = line[tagSeparator+1:]
	case tagSeparator == -1 && digestSeparator != -1:
		// ubuntu@sha256:9b1702...
		name = line[:digestSeparator]
		digest = line[digestSeparator+1+len("sha256:"):]
	case tagSeparator == -1 && digestSeparator == -1:
		// ubuntu
		name = line
		tag = "latest"
	}

	return &Image{Name: name, Tag: tag, Digest: digest}
}

// expandField expands environment variables in a field according to
// global args and build args.
func expandField(
	field string,
	gArgs map[string]string,
	bArgs map[string]string,
) string {
	return os.Expand(field, func(arg string) string {
		gVal, ok := gArgs[arg]
		if !ok {
			return ""
		}

		bArg, ok := bArgs[arg]
		if ok {
			return bArg
		}

		return gVal
	})
}

// dBuildArgs returns build args for Dockerfiles from environment variables if
// set via the flag --dockerfile-env-build-args.
func (g *Generator) dBuildArgs() map[string]string {
	bArgs := map[string]string{}

	if !g.DockerfileEnvBuildArgs {
		return bArgs
	}

	for _, e := range os.Environ() {
		argVal := strings.SplitN(e, "=", 2)
		bArgs[argVal[0]] = argVal[1]
	}

	return bArgs
}

// cBuildArgs returns build args for a docker-compose service if the args key
// in the docker-compose file has a value.
func (g *Generator) cBuildArgs(build verbose) map[string]string {
	bArgs := map[string]string{}

	if build.ArgsWrapper == nil {
		return bArgs
	}

	switch args := build.ArgsWrapper.Args.(type) {
	case argsMap:
		for a, v := range args {
			arg := os.ExpandEnv(a)
			val := os.ExpandEnv(v)
			bArgs[arg] = val
		}
	case argsSlice:
		for _, argValStr := range args {
			argValSl := strings.SplitN(argValStr, "=", 2)
			arg := os.ExpandEnv(argValSl[0])

			const argOnlyLen = 1

			switch len(argValSl) {
			case argOnlyLen:
				bArgs[arg] = os.Getenv(arg)
			default:
				val := os.ExpandEnv(argValSl[1])
				bArgs[arg] = val
			}
		}
	}

	return bArgs
}

// stripQuotes strips valid quotes from an ARG instruction's keys
// and values.
func (g *Generator) stripQuotes(s string) string {
	// Valid in a Dockerfile - any number of quotes if quote is on either side.
	// ARG "IMAGE"="busybox"
	// ARG "IMAGE"""""="busybox"""""""""""""
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimRight(strings.TrimLeft(s, "\""), "\"")
	}

	return s
}

// addErrToBiCh adds an error to a base image channel, ensuring the goroutine
// will not leak if the done channel is closed.
func (g *Generator) addErrToBiCh(
	err error,
	bImCh chan<- *BaseImage,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case bImCh <- &BaseImage{err: err}:
	}
}
