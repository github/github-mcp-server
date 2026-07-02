FROM node:26-alpine@sha256:725aeba2364a9b16beae49e180d83bd597dbd0b15c47f1f28875c290bfd255b9 AS ui-build
WORKDIR /app
COPY ui/package*.json ./ui/
RUN cd ui && npm ci
COPY ui/ ./ui/
# Create output directory and build - vite outputs directly to pkg/github/ui_dist/
RUN mkdir -p ./pkg/github/ui_dist && \
    cd ui && npm run build

FROM golang:1.25.11-alpine@sha256:523c3effe300580ed375e43f43b1c9b091b68e935a7c3a92bfcc4e7ed55b18c2 AS build
ARG VERSION="dev"

# Set the working directory
WORKDIR /build

# Install git
RUN --mount=type=cache,target=/var/cache/apk \
    apk add git

# Copy source code (including ui_dist placeholder)
COPY . .

# Copy built UI assets over the placeholder
COPY --from=ui-build /app/pkg/github/ui_dist/* ./pkg/github/ui_dist/

# Build the server
# OAuth credentials are injected via build secrets so they are not baked into image history; the values are public in practice but kept out of layers.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=oauth_client_id \
    --mount=type=secret,id=oauth_client_secret \
    export OAUTH_CLIENT_ID="$(cat /run/secrets/oauth_client_id 2>/dev/null || echo '')" && \
    export OAUTH_CLIENT_SECRET="$(cat /run/secrets/oauth_client_secret 2>/dev/null || echo '')" && \
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=${OAUTH_CLIENT_ID} -X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientSecret=${OAUTH_CLIENT_SECRET}" \
    -o /bin/github-mcp-server ./cmd/github-mcp-server

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12@sha256:e7e678c88c59e70e105a46549bb3fbfb3d732ee3b4afd3a19fdab2e15afaa6b3

# Add required MCP server annotation
LABEL io.modelcontextprotocol.server.name="io.github.github/github-mcp-server"

# Set the working directory
WORKDIR /server
# Copy the binary from the build stage
COPY --from=build /bin/github-mcp-server .
# Expose the default port
EXPOSE 8082
# Set the entrypoint to the server binary
ENTRYPOINT ["/server/github-mcp-server"]
# Default arguments for ENTRYPOINT
CMD ["stdio"]
