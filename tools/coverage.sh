#!/usr/bin/env bash

(
    cd "$(dirname "$0")/.." || exit
    go test ./unittests -cover -coverpkg ./... -coverprofile=coverage.out
)
