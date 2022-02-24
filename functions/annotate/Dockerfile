### Build executable
FROM golang:1.17 as builder
WORKDIR /workspace

# Copy the Go manifests, download dependencies & cache them before building and copying actual source code, so when
# source code changes, downloaded dependencies stay cached and are not downloaded again (unless manifest changes too.)
COPY go.mod go.sum ./
RUN go mod download

# Now build the actual executable
COPY main.go ./
ENV CGO_ENABLED="0"
ENV GOARCH="amd64"
ENV GOOS="linux"
ENV GO111MODULE="on"
RUN go build -o main main.go

### Target layer
FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /workspace/main ./annotate
ENV GOTRACEBACK=all
ENTRYPOINT ["/annotate"]
