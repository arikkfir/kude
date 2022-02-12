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
RUN go build -o label main.go

### Target layer
#FROM gcr.io/distroless/static:nonroot
FROM alpine:3.15
WORKDIR /
COPY --from=builder /workspace/label ./label
ENV GOTRACEBACK=all
ENTRYPOINT ["/label"]
