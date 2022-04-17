########################################################################################################################
# NOTES:
#   1. This Dockerfile is meant to be run in the root directory of this repository
########################################################################################################################

### Build executable
FROM golang:1.17 as builder
WORKDIR /workspace

# Download "yq"
RUN curl -sSL -o /usr/local/bin/yq https://github.com/mikefarah/yq/releases/download/v4.24.5/yq_linux_amd64 && chmod +x /usr/local/bin/yq

# Copy the Go manifests, download dependencies & cache them before building and copying actual source code, so when
# source code changes, downloaded dependencies stay cached and are not downloaded again (unless manifest changes too.)
COPY go.mod go.sum ./
RUN go mod download

# Now build the actual executable
ARG function
COPY cmd/functions/${function} ./cmd/functions/${function}
COPY pkg ./pkg
ENV CGO_ENABLED="0"
ENV GOARCH="amd64"
ENV GOOS="linux"
ENV GO111MODULE="on"
RUN go build -o function ./cmd/functions/${function}/main.go

### Target layer
FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /workspace/function ./function
COPY --from=builder /usr/local/bin/yq /usr/local/bin/yq
ENV GOTRACEBACK=all
ENTRYPOINT ["/function"]

### Labels
LABEL "kude.kfirs.com/minimum-version"="0.0.0-dev"
