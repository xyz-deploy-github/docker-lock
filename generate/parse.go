package generate

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

type parsedImageLine struct {
	line    string // e.g. python:3.6@sha256:25a189...
	dPath   string
	cPath   string
	pos     int
	svcName string
	err     error
}

func (g *Generator) parseFiles(doneCh <-chan struct{}) chan *parsedImageLine {
	pilCh := make(chan *parsedImageLine)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go g.parseDfiles(pilCh, &wg, doneCh)

	wg.Add(1)

	go g.parseCfiles(pilCh, &wg, doneCh)

	go func() {
		wg.Wait()
		close(pilCh)
	}()

	return pilCh
}

func (g *Generator) parseDfiles(
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	bArgs := map[string]string{}

	if g.DockerfileEnvBuildArgs {
		for _, e := range os.Environ() {
			argVal := strings.SplitN(e, "=", 2)
			bArgs[argVal[0]] = argVal[1]
		}
	}

	for _, dPath := range g.DockerfilePaths {
		wg.Add(1)

		go g.parseDfile(dPath, bArgs, "", "", pilCh, wg, doneCh)
	}
}

func (g *Generator) parseDfile(
	dPath string,
	bArgs map[string]string,
	cPath string,
	svcName string,
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	dfile, err := os.Open(dPath) // nolint: gosec
	if err != nil {
		addErrToPilCh(err, pilCh, doneCh)
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

						strpVar := stripQuotesFromArgInst(vv[varIndex])
						strpVal := stripQuotesFromArgInst(vv[valIndex])
						gArgs[strpVar] = strpVal
					} else {
						// ARG VAR1
						strpVar := stripQuotesFromArgInst(fields[imLineIndex])
						gArgs[strpVar] = ""
					}
				}
			case "from":
				gCtx = false
				line := expandField(fields[imLineIndex], gArgs, bArgs)

				if !stages[line] {
					select {
					case <-doneCh:
						return
					case pilCh <- &parsedImageLine{
						line:    line,
						dPath:   dPath,
						cPath:   cPath,
						svcName: svcName,
						pos:     pos,
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

func (g *Generator) parseCfiles(
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	for _, cPath := range g.ComposefilePaths {
		wg.Add(1)

		go g.parseCfile(cPath, pilCh, wg, doneCh)
	}
}

func (g *Generator) parseCfile(
	cPath string,
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	ymlByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		addErrToPilCh(err, pilCh, doneCh)
		return
	}

	comp := compose{}
	if err := yaml.Unmarshal(ymlByt, &comp); err != nil {
		addErrToPilCh(err, pilCh, doneCh)
		return
	}

	for svcName, svc := range comp.Services {
		wg.Add(1)

		go g.parseSvc(svcName, svc, cPath, pilCh, wg, doneCh)
	}
}

func (g *Generator) parseSvc(
	svcName string,
	svc *service,
	cPath string,
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()

	if svc.BuildWrapper == nil {
		line := os.ExpandEnv(svc.ImageName)
		select {
		case <-doneCh:
		case pilCh <- &parsedImageLine{
			line:    line,
			cPath:   cPath,
			svcName: svcName,
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

		go g.parseDfile(dPath, nil, cPath, svcName, pilCh, wg, doneCh)
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

		bArgs := map[string]string{}

		if build.ArgsWrapper != nil {
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
		}

		wg.Add(1)

		go g.parseDfile(dPath, bArgs, cPath, svcName, pilCh, wg, doneCh)
	}
}

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

		var val string

		bArg, ok := bArgs[arg]
		if ok {
			val = bArg
		} else {
			val = gVal
		}

		return val
	})
}

func stripQuotesFromArgInst(s string) string {
	// Valid in a Dockerfile - any number of quotes if quote is on either side.
	// ARG "IMAGE"="busybox"
	// ARG "IMAGE"""""="busybox"""""""""""""
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimRight(strings.TrimLeft(s, "\""), "\"")
	}

	return s
}

func addErrToPilCh(
	err error,
	pilCh chan<- *parsedImageLine,
	doneCh <-chan struct{},
) {
	select {
	case <-doneCh:
	case pilCh <- &parsedImageLine{err: err}:
	}
}
