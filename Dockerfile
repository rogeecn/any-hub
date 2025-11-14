# syntax=docker/dockerfile:1.7

FROM golang:1.25 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/root/.cache/go-build \
    cd /src \
    ls -l \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X github.com/rogeecn/any-hub/internal/version.Version=${VERSION} -X github.com/rogeecn/any-hub/internal/version.Commit=${COMMIT}" -o /out/any-hub ./cmd/any-hub

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/any-hub /usr/local/bin/any-hub
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/any-hub"]
CMD ["--help"]
