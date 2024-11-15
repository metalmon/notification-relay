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

# Check for required commands
REQUIRED_COMMANDS="docker docker-compose"
for cmd in $REQUIRED_COMMANDS; do
    if ! command_exists "$cmd"; then
        print_message "$RED" "Error: $cmd is not installed"
        exit 1
    fi
done

# Default installation directory
INSTALL_DIR="/opt/notification-relay"
CONFIG_DIR="/etc/notification-relay"
SERVICE_NAME="notification-relay"

# Create directories
create_directory "$INSTALL_DIR"
create_directory "$CONFIG_DIR"

# Copy files to installation directory
print_message "$GREEN" "Copying files to $INSTALL_DIR"
cp -r ./* "$INSTALL_DIR/"

# Create systemd service file
cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Notification Relay Service
After=docker.service
Requires=docker.service

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
Environment=NOTIFICATION_RELAY_CONFIG=${CONFIG_DIR}/config.json
Environment=GOOGLE_APPLICATION_CREDENTIALS=${CONFIG_DIR}/service-account.json
# Default CORS and proxy settings (can be overridden in config)
Environment=TRUSTED_PROXIES=127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
Environment=ALLOWED_ORIGINS=https://your-app.com,https://app.your-domain.com
ExecStart=/usr/bin/docker-compose -f docker-compose.yml -f docker-compose.prod.yml up
ExecStop=/usr/bin/docker-compose -f docker-compose.yml -f docker-compose.prod.yml down
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
    "allowed_origins": [
        "https://your-app.com",
        "https://app.your-domain.com"
    ]
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
echo "3. Set appropriate permissions on configuration files"
echo "4. Configure your allowed origins in config.json or environment variables"
echo "5. Configure your trusted proxies in config.json or environment variables"

# Final permission settings
chown -R root:root "$INSTALL_DIR"
chown -R root:root "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"
if [ -f "${CONFIG_DIR}/service-account.json" ]; then
    chmod 640 "${CONFIG_DIR}/service-account.json"
fi

exit 0 