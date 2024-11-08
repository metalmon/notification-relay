# Build stage
FROM golang:1.21-alpine AS builder
LABEL stage=intermediate

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o notification-relay

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates shadow

# Arguments for user creation
ARG USER_ID=1000
ARG GROUP_ID=1000

# Create frappe group and user with host UID/GID
RUN groupadd -g ${GROUP_ID} frappe && \
    useradd -u ${USER_ID} -g frappe -s /bin/sh -m frappe

# Create config directory with proper ownership
RUN mkdir -p /etc/notification-relay && \
    chown -R ${USER_ID}:${GROUP_ID} /etc/notification-relay && \
    chmod 750 /etc/notification-relay

# Copy binary from builder
COPY --from=builder /app/notification-relay /usr/local/bin/
RUN chmod +x /usr/local/bin/notification-relay

# Use frappe user
USER frappe

# Set default environment variables
ENV NOTIFICATION_RELAY_CONFIG=/etc/notification-relay/config.json
ENV LISTEN_PORT=5000

# Expose port (using environment variable)
EXPOSE ${LISTEN_PORT}

ENTRYPOINT ["/usr/local/bin/notification-relay"] 