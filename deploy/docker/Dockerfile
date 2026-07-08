# syntax=docker/dockerfile:1.7
#
# Multi-stage build for the Go services of the GPU Cluster Scheduler.
# Pick which binary the final image runs with the SERVICE build arg:
#   docker build -f deploy/docker/Dockerfile --build-arg SERVICE=scheduler -t scheduler .
# Valid SERVICE values: api-gateway | scheduler | node-agent | metrics
#
# The builder compiles all four binaries; the runtime stage copies only the
# selected one, keeping the final image small.

########################  builder  ########################
FROM golang:1.22-alpine AS builder

# git for module fetching, ca-certs baked in later stage
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache module downloads
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

ARG SERVICE=api-gateway
ARG VERSION=dev
ARG COMMIT=unknown

# Build all binaries (cached) then keep the one we need.
# CGO disabled -> static binary that runs on scratch/distroless/alpine.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    for svc in api-gateway scheduler node-agent metrics; do \
      CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/${svc} ./cmd/${svc}; \
    done; \
    cp /out/${SERVICE} /out/service

########################  runtime  ########################
# alpine (not scratch/distroless) so container healthchecks can use wget.
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/service /app/service

ARG SERVICE=api-gateway
ENV SERVICE=${SERVICE}

USER app

# Common ports (only the relevant ones per service are actually bound):
#   8080 REST | 50051/50061 gRPC | 9090-9093 metrics
EXPOSE 8080 9090 9091 9092 9093 50051 50061

ENTRYPOINT ["/app/service"]
