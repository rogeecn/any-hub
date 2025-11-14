# syntax=docker/dockerfile:1.7

FROM golang:1.25 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
ARG COMMIT=dev
WORKDIR /src
COPY . .
RUN go mod download \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X github.com/any-hub/any-hub/internal/version.Version=${VERSION} -X github.com/any-hub/any-hub/internal/version.Commit=${COMMIT}" -o /out/any-hub /src/cmd/any-hub

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/any-hub /usr/local/bin/any-hub
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/any-hub"]
CMD ["--help"]
