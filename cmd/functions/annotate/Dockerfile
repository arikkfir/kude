########################################################################################################################
# NOTES:
#   1. This Dockerfile is meant to be run in the root directory of this repository
########################################################################################################################

### Build executable
FROM golang:1.17 as builder
WORKDIR /workspace

# Copy the Go manifests, download dependencies & cache them before building and copying actual source code, so when
# source code changes, downloaded dependencies stay cached and are not downloaded again (unless manifest changes too.)
COPY go.mod go.sum ./
RUN go mod download

# Now build the actual executable
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
ENV CGO_ENABLED="0"
ENV GOARCH="amd64"
ENV GOOS="linux"
ENV GO111MODULE="on"
ARG function
RUN go build -o function ./cmd/functions/${function}/main.go

### Target layer
FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /workspace/function ./function
ENV GOTRACEBACK=all
ENTRYPOINT ["/function"]

### Labels
LABEL "kude.kfirs.com/minimum-version"=0.0.0
