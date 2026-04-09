# syntax=docker/dockerfile:1.4

# Build stage
FROM golang:1.23-alpine AS builder

ARG VERSION=0.1.0
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache git
WORKDIR /app
# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download
# Copy the rest
COPY . .
# Build the server binary with version info
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
      -X github.com/langowarny/smartthings-mcp/internal/version.Version=${VERSION} \
      -X github.com/langowarny/smartthings-mcp/internal/version.Commit=${COMMIT} \
      -X github.com/langowarny/smartthings-mcp/internal/version.Date=${BUILD_DATE}" \
    -o server ./cmd/server

# Final stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server /usr/local/bin/server

LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

EXPOSE 8081

ENTRYPOINT ["server"]

CMD ["-transport", "stream", "-host", "0.0.0.0"]
