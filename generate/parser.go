package generate

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

// parser extracts base images from Dockerfiles and docker-compose files.
type parser struct {
	dPaths            []string
	cPaths            []string
	dfileEnvBuildArgs bool
}

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

// parseFiles finds all base images in the parser's Dockerfiles and
// docker-compose files.
func (p *parser) parseFiles(doneCh <-chan struct{}) chan *BaseImage {
	bImCh := make(chan *BaseImage)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go p.parseDfiles(bImCh, &wg, doneCh)

	wg.Add(1)

	go p.parseCfiles(bImCh, &wg, doneCh)

	go func() {
		wg.Wait()
		close(bImCh)
	}()

	return bImCh
}

// parseDfiles finds base images in Dockerfiles.
func (p *parser) parseDfiles(
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	bArgs := p.dBuildArgs()

	for _, dPath := range p.dPaths {
		wg.Add(1)

		go p.parseDfile(dPath, bArgs, "", "", bImCh, wg, doneCh)
	}
}

// parseDfile finds base images in a Dockerfile.
func (p *parser) parseDfile(
	dPath string,
	bArgs map[string]string,
	cPath string,
	svcName string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	log.Printf("Parsing '%s' with build args '%v'.", dPath, bArgs)

	dfile, err := os.Open(dPath) // nolint: gosec
	if err != nil {
		p.addErrToBiCh(err, bImCh, doneCh)
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
					log.Printf("In '%s', found global ARG fields '%v' before "+
						"the first FROM instruction.", dPath, fields,
					)

					if strings.Contains(fields[imLineIndex], "=") {
						//ARG VAR=VAL
						vv := strings.SplitN(fields[imLineIndex], "=", 2)

						const varIndex = 0

						const valIndex = 1

						strpVar := p.stripQuotes(vv[varIndex])
						strpVal := p.stripQuotes(vv[valIndex])

						gArgs[strpVar] = strpVal

						log.Printf("In '%s', set global ARG '%s' to '%s'.",
							dPath, strpVar, strpVal,
						)
					} else {
						// ARG VAR1
						strpVar := p.stripQuotes(fields[imLineIndex])

						gArgs[strpVar] = ""

						log.Printf("In '%s', set global ARG '%s' to '\"\"'",
							dPath, strpVar,
						)
					}
				}
			case "from":
				gCtx = false

				log.Printf("In '%s', found FROM fields '%v'.", dPath, fields)

				line := fields[imLineIndex]

				if !stages[line] {
					log.Printf("In '%s', line '%s' has not been previously "+
						"declared as a build stage.", dPath, line,
					)

					line = expandField(dPath, fields[imLineIndex], gArgs, bArgs)

					log.Printf("In '%s', expanded line to '%s'", dPath, line)

					im := p.convertImageLineToImage(line)

					log.Printf("In '%s', converted '%s' to image '%+v'.",
						dPath, line, im,
					)

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
					stage := fields[stageIndex]

					log.Printf("In '%s', found new build stage '%s'.",
						dPath, stage,
					)

					stages[stage] = true
				}
			}
		}
	}
}

// parseCfiles finds base images in docker-compose files.
func (p *parser) parseCfiles(
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	for _, cPath := range p.cPaths {
		wg.Add(1)

		go p.parseCfile(cPath, bImCh, wg, doneCh)
	}
}

// parseCfile finds base images in a docker-compose file.
func (p *parser) parseCfile(
	cPath string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	log.Printf("Parsing '%s'.", cPath)

	ymlByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		p.addErrToBiCh(err, bImCh, doneCh)
		return
	}

	comp := compose{}
	if err := yaml.Unmarshal(ymlByt, &comp); err != nil {
		err = fmt.Errorf("from '%s': %v", cPath, err)
		p.addErrToBiCh(err, bImCh, doneCh)

		return
	}

	for svcName, svc := range comp.Services {
		wg.Add(1)

		go p.parseService(svcName, svc, cPath, bImCh, wg, doneCh)
	}
}

// parseService finds base images in a service.
func (p *parser) parseService(
	svcName string,
	svc *service,
	cPath string,
	bImCh chan<- *BaseImage,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	log.Printf("In '%s', parsing service '%s'", cPath, svcName)

	if svc.BuildWrapper == nil {
		log.Printf("In '%s', service '%s' has an image key with value '%s'.",
			cPath, svcName, svc.ImageName,
		)

		imLine := os.ExpandEnv(svc.ImageName)

		log.Printf("In '%s', service '%s' expanded image line '%s' to '%s'.",
			cPath, svcName, svc.ImageName, imLine,
		)

		im := p.convertImageLineToImage(imLine)

		log.Printf("In '%s', service '%s' converted '%s' to image '%+v'.",
			cPath, svcName, imLine, im,
		)

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

		log.Printf("In '%s', service '%s' Dockerfile '%s' found.",
			cPath, svcName, dPath,
		)

		wg.Add(1)

		go p.parseDfile(dPath, nil, cPath, svcName, bImCh, wg, doneCh)
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

		bArgs := p.cBuildArgs(build)

		log.Printf("In '%s', service '%s' Dockerfile '%s' "+
			"found with build args '%+v'.", cPath, svcName, dPath, bArgs,
		)

		wg.Add(1)

		go p.parseDfile(dPath, bArgs, cPath, svcName, bImCh, wg, doneCh)
	}
}

// convertLineToIm converts a line such as ubuntu:bionic into an Image.
func (p *parser) convertImageLineToImage(line string) *Image {
	tagSeparator := -1
	digestSeparator := -1

loop:
	for i, c := range line {
		switch c {
		case ':':
			tagSeparator = i
		case '/':
			// reset tagSeparator
			// for instance, 'localhost:5000/my-image'
			tagSeparator = -1
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
	dPath string,
	field string,
	gArgs map[string]string,
	bArgs map[string]string,
) string {
	return os.Expand(field, func(arg string) string {
		gVal, ok := gArgs[arg]
		if !ok {
			log.Printf("In '%s', '%s' not found in global args or build args. "+
				"Using \"\" as the value.", dPath, arg,
			)
			return ""
		}

		log.Printf("In '%s', ARG '%s' found in global args with value '%s'.",
			dPath, arg, gVal,
		)

		bVal, ok := bArgs[arg]
		if !ok {
			log.Printf("In '%s', ARG '%s' not found in build args, "+
				"using global value '%s'.", dPath, arg, gVal,
			)

			return gVal
		}
		log.Printf("In '%s', ARG '%s' found in build args. "+
			"Overriding global value with build value '%s'.",
			dPath, arg, bVal,
		)
		return bVal
	})
}

// dBuildArgs returns build args for Dockerfiles from environment variables if
// set via the flag --dockerfile-env-build-args.
func (p *parser) dBuildArgs() map[string]string {
	bArgs := map[string]string{}

	if !p.dfileEnvBuildArgs {
		return bArgs
	}

	log.Printf("Using environment variables as build args for all Dockerfiles.")

	for _, e := range os.Environ() {
		argVal := strings.SplitN(e, "=", 2)
		bArgs[argVal[0]] = argVal[1]
	}

	return bArgs
}

// cBuildArgs returns build args for a docker-compose service if the args key
// in the docker-compose file has a value.
func (p *parser) cBuildArgs(build verbose) map[string]string {
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
func (p *parser) stripQuotes(s string) string {
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
func (p *parser) addErrToBiCh(
	err error,
	bImCh chan<- *BaseImage,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case bImCh <- &BaseImage{err: err}:
	}
}
