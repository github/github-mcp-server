FROM golang:1.24.4-alpine AS build
ARG VERSION="dev"

# Set the working directory
WORKDIR /build

# Install git
RUN --mount=type=cache,target=/var/cache/apk \
    apk add git

# Prepare build_info files
RUN --mount=type=bind,target=. \
    mkdir -p cmd/github-mcp-server/build_info && \
    git rev-parse HEAD > cmd/github-mcp-server/build_info/commit.txt && \
    date -u +%Y-%m-%dT%H:%M:%SZ > cmd/github-mcp-server/build_info/date.txt && \
    echo "${VERSION}" > cmd/github-mcp-server/build_info/version.txt

# Build the server
# go build automatically download required module dependencies to /go/pkg/mod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -ldflags="-s -w" \
    -o /bin/github-mcp-server cmd/github-mcp-server/main.go

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12
# Set the working directory
WORKDIR /server
# Copy the binary from the build stage
COPY --from=build /bin/github-mcp-server .
# Set the entrypoint to the server binary
ENTRYPOINT ["/server/github-mcp-server"]
# Default arguments for ENTRYPOINT
CMD ["stdio"]
