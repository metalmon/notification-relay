#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to create directory with proper permissions
create_directory() {
    local dir=$1
    if [ ! -d "$dir" ]; then
        print_message "$YELLOW" "Creating directory: $dir"
        mkdir -p "$dir"
        chmod 750 "$dir"
    fi
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_message "$RED" "Please run as root"
    exit 1
fi

# Check for Docker
if ! command_exists "docker"; then
    print_message "$RED" "Error: Docker is not installed"
    exit 1
fi

# Check for Docker Compose
COMPOSE_CMD=""
if docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
    print_message "$GREEN" "Using: docker compose"
elif command_exists "docker-compose"; then
    COMPOSE_CMD="docker-compose"
    print_message "$GREEN" "Using: docker-compose"
else
    print_message "$RED" "Error: neither 'docker compose' nor 'docker-compose' is available"
    exit 1
fi

# Default installation directory
INSTALL_DIR="/opt/notification-relay"
CONFIG_DIR="/etc/notification-relay"
SERVICE_NAME="notification-relay"
REPO_URL="https://raw.githubusercontent.com/metalmon/notification-relay/main"

# Create directories
create_directory "$INSTALL_DIR"
create_directory "$CONFIG_DIR"

# Download only necessary files if installing via curl
print_message "$GREEN" "Downloading required files to $INSTALL_DIR"

# Download docker-compose files
curl -sSL "$REPO_URL/docker-compose.yml" -o "$INSTALL_DIR/docker-compose.yml"
curl -sSL "$REPO_URL/docker-compose.prod.yml" -o "$INSTALL_DIR/docker-compose.prod.yml"
curl -sSL "$REPO_URL/.env.example" -o "$INSTALL_DIR/.env.example"

# Use default values - non-interactive for stability
print_message "$GREEN" "Using default configuration values. You can modify them later in $INSTALL_DIR/.env"

# Set default values
PUSH_DOMAIN="push.example.com"
ALLOWED_ORIGINS="*"
print_message "$YELLOW" "Using default domain: $PUSH_DOMAIN"
print_message "$YELLOW" "Using '*' for allowed origins. Not recommended for production!"

# Create .env file
cat > "$INSTALL_DIR/.env" << EOF
# Server Configuration
LISTEN_PORT=5000
NOTIFICATION_RELAY_CONFIG=${CONFIG_DIR}/config.json
GOOGLE_APPLICATION_CREDENTIALS=${CONFIG_DIR}/service-account.json

# Proxy Configuration
TRUSTED_PROXIES=127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# CORS Configuration
ALLOWED_ORIGINS=${ALLOWED_ORIGINS}

# Traefik Configuration
PUSH_DOMAIN=${PUSH_DOMAIN}
CERT_RESOLVER=le

# Docker Configuration
CONFIG_DIR=${CONFIG_DIR}
REPLICAS=2
LOG_MAX_SIZE=10m
LOG_MAX_FILES=3

# User/Group IDs
DOCKER_UID=1000
DOCKER_GID=1000
EOF

# Create systemd service file
cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Notification Relay Service
After=docker.service
Requires=docker.service

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${COMPOSE_CMD} --env-file ${INSTALL_DIR}/.env -f docker-compose.yml -f docker-compose.prod.yml up
ExecStop=${COMPOSE_CMD} --env-file ${INSTALL_DIR}/.env -f docker-compose.yml -f docker-compose.prod.yml down
Restart=always
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF

# Set proper permissions
chmod 644 "/etc/systemd/system/${SERVICE_NAME}.service"

# Check for config.json
if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    print_message "$YELLOW" "Creating example config.json"
    cat > "${CONFIG_DIR}/config.json" << EOF
{
    "projects": {
        "your-project": {
            "vapid_public_key": "your-vapid-public-key",
            "firebase_config": {
                "apiKey": "your-firebase-api-key",
                "authDomain": "your-project.firebaseapp.com",
                "projectId": "your-project-id",
                "storageBucket": "your-project.appspot.com",
                "messagingSenderId": "your-sender-id",
                "appId": "your-app-id",
                "measurementId": "your-measurement-id"
            }
        }
    },
    "trusted_proxies": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16",
    "allowed_origins": []
}
EOF
    chmod 640 "${CONFIG_DIR}/config.json"
    print_message "$YELLOW" "Please update ${CONFIG_DIR}/config.json with your configuration"
fi

# Create development override file
cat > "${INSTALL_DIR}/docker-compose.override.yml" << EOF
version: '3.8'

services:
  notification-relay:
    environment:
      - TRUSTED_PROXIES=*
      - ALLOWED_ORIGINS=http://localhost:8000,https://*.app.github.dev
    volumes:
      - ./config:/etc/notification-relay
      - .:/app
    command: ["./notification-relay"]
EOF

# Check for service account file
if [ ! -f "${CONFIG_DIR}/service-account.json" ]; then
    print_message "$YELLOW" "Warning: service-account.json not found in ${CONFIG_DIR}"
    print_message "$YELLOW" "Please copy your Firebase service account key to ${CONFIG_DIR}/service-account.json"
    print_message "$YELLOW" "After copying, set proper permissions with: chmod 640 ${CONFIG_DIR}/service-account.json"
fi

# Reload systemd
systemctl daemon-reload

print_message "$GREEN" "Installation completed!"
print_message "$GREEN" "To start the service:"
echo "systemctl start $SERVICE_NAME"
print_message "$GREEN" "To enable service on boot:"
echo "systemctl enable $SERVICE_NAME"
print_message "$YELLOW" "Remember to:"
echo "1. Update ${CONFIG_DIR}/config.json with your configuration"
echo "2. Copy your service account key to ${CONFIG_DIR}/service-account.json"
echo "3. Update ${INSTALL_DIR}/.env with your domain settings"

# Final permission settings
chown -R root:root "$INSTALL_DIR"
chown -R root:root "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"
if [ -f "${CONFIG_DIR}/service-account.json" ]; then
    chmod 640 "${CONFIG_DIR}/service-account.json"
fi

exit 0 