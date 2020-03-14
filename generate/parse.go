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
	var wg sync.WaitGroup
	wg.Add(1)
	go g.parseDockerfiles(pilCh, &wg, doneCh)
	wg.Add(1)
	go g.parseComposefiles(pilCh, &wg, doneCh)
	go func() {
		wg.Wait()
		close(pilCh)
	}()
	return pilCh
}

func (g *Generator) parseDockerfiles(
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
		go g.parseDockerfile(dPath, bArgs, "", "", pilCh, wg, doneCh)
	}
}

func (g *Generator) parseDockerfile(
	dPath string,
	bArgs map[string]string,
	cPath string,
	svcName string,
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	dFile, err := os.Open(dPath) // nolint: gosec
	if err != nil {
		select {
		case <-doneCh:
		case pilCh <- &parsedImageLine{err: err}:
		}
		return
	}
	defer dFile.Close()
	var pos int                  // order of image line in Dockerfile
	stages := map[string]bool{}  // FROM <image line> as <stage>
	gArgs := map[string]string{} // global ARGs before the first FROM
	gCtx := true                 // true if before first FROM
	scnr := bufio.NewScanner(dFile)
	scnr.Split(bufio.ScanLines)
	for scnr.Scan() {
		fields := strings.Fields(scnr.Text())
		if len(fields) > 0 {
			switch instruction := strings.ToLower(fields[0]); instruction {
			case "arg":
				if gCtx {
					if strings.Contains(fields[1], "=") {
						//ARG VAR=VAL
						vv := strings.SplitN(fields[1], "=", 2)
						strpVar := stripQuotesFromArgInstruction(vv[0])
						strpVal := stripQuotesFromArgInstruction(vv[1])
						gArgs[strpVar] = strpVal
					} else {
						// ARG VAR1
						strpVar := stripQuotesFromArgInstruction(fields[1])
						gArgs[strpVar] = ""
					}
				}
			case "from":
				gCtx = false
				line := expandField(fields[1], gArgs, bArgs)
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
				if len(fields) == 4 {
					stage := expandField(fields[3], gArgs, bArgs)
					stages[stage] = true
				}
			}
		}
	}
}

func (g *Generator) parseComposefiles(
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	for _, cPath := range g.ComposefilePaths {
		wg.Add(1)
		go g.parseComposefile(cPath, pilCh, wg, doneCh)
	}
}

func (g *Generator) parseComposefile(
	cPath string,
	pilCh chan<- *parsedImageLine,
	wg *sync.WaitGroup,
	doneCh <-chan struct{},
) {
	defer wg.Done()
	ymlByt, err := ioutil.ReadFile(cPath) // nolint: gosec
	if err != nil {
		select {
		case <-doneCh:
		case pilCh <- &parsedImageLine{err: err}:
		}
		return
	}
	var comp compose
	if err := yaml.Unmarshal(ymlByt, &comp); err != nil {
		select {
		case <-doneCh:
		case pilCh <- &parsedImageLine{err: err}:
		}
		return
	}
	for svcName, svc := range comp.Services {
		wg.Add(1)
		go g.parseService(svcName, svc, cPath, pilCh, wg, doneCh)
	}
}

func (g *Generator) parseService(
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
		dDir := filepath.ToSlash(os.ExpandEnv(string(build)))
		dPath := filepath.ToSlash(filepath.Join(dDir, "Dockerfile"))
		dPath, err := normalizeDPath(dPath, cPath)
		if err != nil {
			select {
			case <-doneCh:
			case pilCh <- &parsedImageLine{err: err}:
			}
			return
		}
		wg.Add(1)
		go g.parseDockerfile(dPath, nil, cPath, svcName, pilCh, wg, doneCh)
	case verbose:
		ctx := filepath.ToSlash(os.ExpandEnv(build.Context))
		dPath := filepath.ToSlash(os.ExpandEnv(build.DockerfilePath))
		if dPath == "" {
			dPath = "Dockerfile"
		}
		dPath = filepath.ToSlash(filepath.Join(ctx, dPath))
		dPath, err := normalizeDPath(dPath, cPath)
		if err != nil {
			select {
			case <-doneCh:
			case pilCh <- &parsedImageLine{err: err}:
			}
			return
		}
		bArgs := map[string]string{}
		for _, argVal := range build.Args {
			av := strings.SplitN(argVal, "=", 2)
			bArgs[os.ExpandEnv(av[0])] = os.ExpandEnv(av[1])
		}
		wg.Add(1)
		go g.parseDockerfile(dPath, bArgs, cPath, svcName, pilCh, wg, doneCh)
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

func stripQuotesFromArgInstruction(s string) string {
	// Valid in a Dockerfile - any number of quotes if quote is on either side.
	// ARG "IMAGE"="busybox"
	// ARG "IMAGE"""""="busybox"""""""""""""
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimRight(strings.TrimLeft(s, "\""), "\"")
	}
	return s
}

func normalizeDPath(dPath string, cPath string) (string, error) {
	if filepath.IsAbs(dPath) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		wd = filepath.ToSlash(wd)
		if strings.HasPrefix(dPath, wd) {
			dPath = filepath.ToSlash(
				filepath.Join(".", strings.TrimPrefix(dPath, wd)),
			)
		} else {
			return "",
				fmt.Errorf("%s is outside the current working directory", dPath)
		}
	} else {
		dPath = filepath.ToSlash(filepath.Join(filepath.Dir(cPath), dPath))
	}
	if strings.HasPrefix(dPath, "..") {
		return "",
			fmt.Errorf("%s is outside the current working directory", dPath)
	}
	return dPath, nil
}
