FROM mcr.microsoft.com/devcontainers/go:latest

ENV LANG en_US.utf8
ENV DEBIAN_FRONTEND=noninteractive

# Set GOPATH for vscode user
ENV GOPATH=/home/vscode/go
ENV GOBIN=/home/vscode/go/bin
ENV PATH=$PATH:$GOBIN

# Create GOPATH directories with correct permissions
RUN mkdir -p /home/vscode/go/bin && \
    chown -R vscode:vscode /home/vscode/go

# Install golangci-lint as vscode user
ARG GOLANGCI_LINT_VERSION=latest
RUN su vscode -c "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /home/vscode/go/bin ${GOLANGCI_LINT_VERSION}"

# Install Go development tools as vscode user
RUN su vscode -c "go install golang.org/x/tools/gopls@latest" && \
    su vscode -c "go install github.com/go-delve/delve/cmd/dlv@latest" && \
    su vscode -c "go install honnef.co/go/tools/cmd/staticcheck@latest" && \
    su vscode -c "go install golang.org/x/tools/cmd/goimports@latest" && \
    su vscode -c "go install github.com/fatih/gomodifytags@latest" && \
    su vscode -c "go install github.com/josharian/impl@latest" && \
    su vscode -c "go install github.com/cweill/gotests/gotests@latest" && \
    su vscode -c "go install github.com/vektra/mockery/v2@latest" && \
    su vscode -c "go install github.com/segmentio/golines@latest"

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

