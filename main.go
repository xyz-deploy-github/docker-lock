// Package docker-lock is a cli-plugin for docker that automates managing
// image digests by tracking them in a separate Lockfile (think
// package-lock.json or Pipfile.lock) -
// with docker-lock, you can refer to images in Dockerfiles or
// docker-compose files by mutable tags (as in python:3.6)
// yet receive the same benefits as if you had specified immutable digests
// (as in python:3.6@sha256:25a189a536...).
//
// To build docker-lock from source on unix, run:
// 	mkdir -p ~/.docker/cli-plugins
//	go build
//	chmod +x docker-lock
//	mv docker-lock ~/.docker/cli-plugins
//
// To build docker-lock from source on windows, create the folder
// 	%USERPROFILE%\.docker\cli-plugins
// and run:
//	go build -o %USERPROFILE%\.docker\cli-plugins\docker-lock.exe
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/safe-waters/docker-lock/cmd"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "docker-cli-plugin-metadata" {
		m := map[string]string{
			"SchemaVersion":    "0.1.0",
			"Vendor":           "https://github.com/safe-waters/docker-lock",
			"Version":          cmd.Version,
			"ShortDescription": "Manage Lockfiles",
		}
		j, _ := json.Marshal(m)

		fmt.Println(string(j))

		os.Exit(0)
	}

	if err := cmd.Execute(nil); err != nil {
		fmt.Fprint(os.Stderr, err)

		fmt.Println()

		os.Exit(1)
	}
}
