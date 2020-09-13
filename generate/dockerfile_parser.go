package generate

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// DockerfileParser extracts image values from Dockerfiles.
type DockerfileParser struct{}

// IDockerfileParser provides an interface for DockerfileParser's exported
// methods, which are used by Generator.
type IDockerfileParser interface {
	ParseFiles(
		pathResults <-chan *PathResult,
		done <-chan struct{},
	) <-chan *DockerfileImage
}

// Image contains information extracted from 'FROM' instructions in Dockerfiles
// or 'image:' keys in docker-compose files. For instance,
// FROM busybox:latest@sha256:dd97a3f...
// could be represented as:
// Image{Name: busybox, Tag: latest, Digest: dd97a3f...}.
type Image struct {
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Digest string `json:"digest"`
}

// DockerfileImage annotates an image with data about the Dockerfile
// from which it was parsed.
type DockerfileImage struct {
	*Image
	Position int    `json:"-"`
	Path     string `json:"-"`
	Err      error  `json:"-"`
}

// ParseFiles reads a Dockerfile to parse all images in FROM instructions.
func (d *DockerfileParser) ParseFiles(
	pathResults <-chan *PathResult,
	done <-chan struct{},
) <-chan *DockerfileImage {
	dockerfileImages := make(chan *DockerfileImage)

	var waitGroup sync.WaitGroup

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		if pathResults == nil {
			return
		}

		for pathResult := range pathResults {
			if pathResult.Err != nil {
				select {
				case <-done:
				case dockerfileImages <- &DockerfileImage{Err: pathResult.Err}:
				}

				return
			}

			waitGroup.Add(1)

			go d.parseFile(
				pathResult.Path, nil, dockerfileImages, done, &waitGroup,
			)
		}
	}()

	go func() {
		waitGroup.Wait()
		close(dockerfileImages)
	}()

	return dockerfileImages
}

func (d *DockerfileParser) parseFile(
	path string,
	buildArgs map[string]string,
	dockerfileImages chan<- *DockerfileImage,
	done <-chan struct{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	f, err := os.Open(path)
	if err != nil {
		select {
		case <-done:
		case dockerfileImages <- &DockerfileImage{Err: err}:
		}

		return
	}
	defer f.Close()

	position := 0                     // order of image in Dockerfile
	stages := map[string]bool{}       // FROM <image line> as <stage>
	globalArgs := map[string]string{} // ARGs before the first FROM
	globalContext := true             // true if before first FROM
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		const instructionIndex = 0

		const imageLineIndex = 1

		if len(fields) > 0 {
			switch strings.ToLower(fields[instructionIndex]) {
			case "arg":
				if globalContext {
					if strings.Contains(fields[imageLineIndex], "=") {
						// ARG VAR=VAL
						varVal := strings.SplitN(fields[imageLineIndex], "=", 2)

						const varIndex = 0

						const valIndex = 1

						strippedVar := d.stripQuotes(varVal[varIndex])
						strippedVal := d.stripQuotes(varVal[valIndex])

						globalArgs[strippedVar] = strippedVal
					} else {
						// ARG VAR1
						strippedVar := d.stripQuotes(fields[imageLineIndex])

						globalArgs[strippedVar] = ""
					}
				}
			case "from":
				globalContext = false

				imageLine := fields[imageLineIndex]

				if !stages[imageLine] {
					imageLine = expandField(imageLine, globalArgs, buildArgs)

					image := convertImageLineToImage(imageLine)

					select {
					case <-done:
						return
					case dockerfileImages <- &DockerfileImage{
						Image: image, Position: position, Path: path,
					}:
						position++
					}
				}

				// FROM <image> AS <stage>
				// FROM <stage> AS <another stage>
				const maxNumFields = 4
				if len(fields) == maxNumFields {
					const stageIndex = 3

					stage := fields[stageIndex]
					stages[stage] = true
				}
			}
		}
	}
}

func (d *DockerfileParser) stripQuotes(s string) string {
	// Valid in a Dockerfile - any number of quotes if quote is on either side.
	// ARG "IMAGE"="busybox"
	// ARG "IMAGE"""""="busybox"""""""""""""
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimRight(strings.TrimLeft(s, "\""), "\"")
	}

	return s
}

func convertImageLineToImage(imageLine string) *Image {
	tagSeparator := -1
	digestSeparator := -1

loop:
	for i, c := range imageLine {
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

	switch {
	case tagSeparator != -1 && digestSeparator != -1:
		// ubuntu:18.04@sha256:9b1702...
		name = imageLine[:tagSeparator]
		tag = imageLine[tagSeparator+1 : digestSeparator]
		digest = imageLine[digestSeparator+1+len("sha256:"):]
	case tagSeparator != -1 && digestSeparator == -1:
		// ubuntu:18.04
		name = imageLine[:tagSeparator]
		tag = imageLine[tagSeparator+1:]
	case tagSeparator == -1 && digestSeparator != -1:
		// ubuntu@sha256:9b1702...
		name = imageLine[:digestSeparator]
		digest = imageLine[digestSeparator+1+len("sha256:"):]
	default:
		// ubuntu
		name = imageLine
		tag = "latest"
	}

	return &Image{Name: name, Tag: tag, Digest: digest}
}

func expandField(
	field string,
	globalArgs map[string]string,
	buildArgs map[string]string,
) string {
	return os.Expand(field, func(arg string) string {
		globalVal, ok := globalArgs[arg]
		if !ok {
			return ""
		}

		buildVal, ok := buildArgs[arg]
		if !ok {
			return globalVal
		}

		return buildVal
	})
}
