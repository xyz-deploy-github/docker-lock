FROM ubuntu:bionic
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG GO_VERSION=1.13
ARG DOCKER_COMPOSE_VERSION=1.25.0
ARG GOLANGCI_LINT_VERSION=1.21.0
RUN apt-get update && \
    apt-get install curl gcc git vim shellcheck -y && \
    curl -fsSL https://get.docker.com | bash && \
    curl -fSsL "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose && \
    chmod +x /usr/local/bin/docker-compose && \
    curl -fSsL "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" | tar -xz -C /usr/local && \
    curl -fSsL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v${GOLANGCI_LINT_VERSION} && \
    rm -rf /var/lib/apt/lists/*
ENV PATH="${PATH}:/usr/local/go/bin:$(go env GOPATH)/bin"
WORKDIR /workspaces/docker-lock