# Stage 1: Build
FROM golang:1.24-alpine AS builder

# Install ca-certificates for TLS (needed for Anthropic/OpenAI API calls)
RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Cache module downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /chainforge-server ./cmd/server/

# Stage 2: Runtime — distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary and certs from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /chainforge-server /chainforge-server

# Use non-root user (uid 65532 = "nonroot" in distroless)
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/chainforge-server"]
