FROM node:20-alpine@sha256:09e2b3d9726018aecf269bd35325f46bf75046a643a66d28360ec71132750ec8 AS ui-build
WORKDIR /app
COPY ui/package*.json ./ui/
RUN cd ui && npm ci
COPY ui/ ./ui/
# Create output directory and build - vite outputs directly to pkg/github/ui_dist/
RUN mkdir -p ./pkg/github/ui_dist && \
    cd ui && npm run build

FROM golang:1.25.9-alpine@sha256:04d017a27c481185c169884328a5761d052910fdced8c3b8edd686474efdf59b AS build
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
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/github-mcp-server ./cmd/github-mcp-server

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12@sha256:9dce90e688a57e59ce473ff7bc4c80bc8fe52d2303b4d99b44f297310bbd2210

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
