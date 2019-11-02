package generate

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/michaelperel/docker-lock/generate/internal/parse"
	"gopkg.in/yaml.v2"
)

type imageLine struct {
	line    string // e.g. python:3.6@sha256:25a189a536ae4d7c77dd5d0929da73057b85555d6b6f8a66bfbcc1a7a7de094b
	dPath   string
	cPath   string
	pos     int
	svcName string
	err     error
}

func (g *Generator) parseFiles() chan *imageLine {
	var (
		ilCh = make(chan *imageLine)
		wg   sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.parseDockerfiles(ilCh)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.parseComposefiles(ilCh)
	}()
	go func() {
		wg.Wait()
		close(ilCh)
	}()
	return ilCh
}

func (g *Generator) parseDockerfiles(ilCh chan<- *imageLine) {
	var (
		bArgs = map[string]string{}
		wg    sync.WaitGroup
	)
	if g.DockerfileEnvBuildArgs {
		for _, e := range os.Environ() {
			argVal := strings.SplitN(e, "=", 2)
			bArgs[argVal[0]] = argVal[1]
		}
	}
	for _, dPath := range g.DockerfilePaths {
		wg.Add(1)
		go func(dPath string) {
			defer wg.Done()
			g.parseDockerfile(dPath, bArgs, "", "", ilCh)
		}(dPath)
	}
	wg.Wait()
}

func (g *Generator) parseDockerfile(dPath string,
	bArgs map[string]string,
	cPath string,
	svcName string,
	ilCh chan<- *imageLine) {

	dFile, err := os.Open(dPath)
	if err != nil {
		ilCh <- &imageLine{err: err}
		return
	}
	defer dFile.Close()
	var (
		pos    int                   // order of image line in Dockerfile
		stages = map[string]bool{}   // FROM <image line> as <stage>
		gArgs  = map[string]string{} // global ARGs before the first FROM
		gCtx   = true                // true if before first FROM
		scnr   = bufio.NewScanner(dFile)
	)
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
					ilCh <- &imageLine{line: line,
						dPath:   dPath,
						cPath:   cPath,
						svcName: svcName,
						pos:     pos}
					pos++
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

func (g *Generator) parseComposefiles(ilCh chan<- *imageLine) {
	var wg sync.WaitGroup
	for _, cPath := range g.ComposefilePaths {
		wg.Add(1)
		go func(cPath string) {
			defer wg.Done()
			g.parseComposefile(cPath, ilCh)
		}(cPath)
	}
	wg.Wait()
}

func (g *Generator) parseComposefile(cPath string, ilCh chan<- *imageLine) {
	ymlByt, err := ioutil.ReadFile(cPath)
	if err != nil {
		ilCh <- &imageLine{err: err}
		return
	}
	var comp parse.Compose
	if err := yaml.Unmarshal(ymlByt, &comp); err != nil {
		ilCh <- &imageLine{err: err}
		return
	}
	var wg sync.WaitGroup
	for svcName, svc := range comp.Services {
		wg.Add(1)
		go func(svcName string, svc *parse.Service) {
			defer wg.Done()
			g.parseService(svcName, svc, cPath, ilCh)
		}(svcName, svc)
	}
	wg.Wait()
}

func (g *Generator) parseService(svcName string, svc *parse.Service, cPath string, ilCh chan<- *imageLine) {
	if svc.BuildWrapper == nil {
		line := os.ExpandEnv(svc.ImageName)
		ilCh <- &imageLine{line: line, cPath: cPath, svcName: svcName}
		return
	}
	switch build := svc.BuildWrapper.Build.(type) {
	case parse.Simple:
		dDir := filepath.ToSlash(os.ExpandEnv(string(build)))
		dPath := filepath.ToSlash(filepath.Join(dDir, "Dockerfile"))
		dPath, err := normalizeDPath(dPath, cPath)
		if err != nil {
			ilCh <- &imageLine{err: err}
			return
		}
		g.parseDockerfile(dPath, nil, cPath, svcName, ilCh)
	case parse.Verbose:
		ctx := filepath.ToSlash(os.ExpandEnv(build.Context))
		dPath := filepath.ToSlash(os.ExpandEnv(build.DockerfilePath))
		if dPath == "" {
			dPath = "Dockerfile"
		}
		dPath = filepath.ToSlash(filepath.Join(ctx, dPath))
		dPath, err := normalizeDPath(dPath, cPath)
		if err != nil {
			ilCh <- &imageLine{err: err}
			return
		}
		bArgs := map[string]string{}
		for _, argVal := range build.Args {
			av := strings.SplitN(argVal, "=", 2)
			bArgs[os.ExpandEnv(av[0])] = os.ExpandEnv(av[1])
		}
		g.parseDockerfile(dPath, bArgs, cPath, svcName, ilCh)
	}
}

func expandField(field string, gArgs map[string]string, bArgs map[string]string) string {
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
			dPath = filepath.ToSlash(filepath.Join(".", strings.TrimPrefix(dPath, wd)))
		} else {
			return "", fmt.Errorf("%s is outside the current working directory", dPath)
		}
	} else {
		dPath = filepath.ToSlash(filepath.Join(filepath.Dir(cPath), dPath))
	}
	if strings.HasPrefix(dPath, "..") {
		return "", fmt.Errorf("%s is outside the current working directory", dPath)
	}
	return dPath, nil
}
