FROM mcr.microsoft.com/devcontainers/go:latest

ENV LANG en_US.utf8
ENV DEBIAN_FRONTEND=noninteractive

# Install golangci-lint
ARG GOLANGCI_LINT_VERSION=latest
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}

# Install Go development tools
RUN go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/fatih/gomodifytags@latest && \
    go install github.com/josharian/impl@latest && \
    go install github.com/cweill/gotests/gotests@latest

# Utilities
RUN apt-get update && apt-get install -y \
    ca-certificates \
    wget \
    curl \
    jq \
    vim \
    netcat-traditional \
    npm \
    git \
    make \
    bash-completion && \
    apt-get autoremove -y && \
    apt-get clean autoclean && \
    rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Set up bash completion for Go tools
RUN echo 'source /usr/share/bash-completion/bash_completion' >> /home/vscode/.bashrc

# Clean up
RUN rm -rf /tmp/*

