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

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_message "$RED" "Please run as root"
    exit 1
fi

# Default paths
BINARY_PATH="/usr/local/bin/notification-relay"
CONFIG_DIR="/etc/notification-relay"
SERVICE_NAME="notification-relay"
GITHUB_REPO="your-org/notification-relay"

# Get latest release version from GitHub
VERSION=$(curl -s https://api.github.com/repos/${GITHUB_REPO}/releases/latest | grep '"tag_name":' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
    print_message "$RED" "Failed to get latest version"
    exit 1
fi

# Create config directory
mkdir -p "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"

# Download binary
print_message "$GREEN" "Downloading notification-relay ${VERSION}..."
curl -L "https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/notification-relay-linux-amd64" -o "$BINARY_PATH"
chmod 755 "$BINARY_PATH"

if [ ! -f "$BINARY_PATH" ]; then
    print_message "$RED" "Failed to download binary"
    exit 1
fi

# Create systemd service file
cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Notification Relay Service
After=network.target

[Service]
Type=simple
Environment=NOTIFICATION_RELAY_CONFIG=${CONFIG_DIR}/config.json
Environment=GOOGLE_APPLICATION_CREDENTIALS=${CONFIG_DIR}/service-account.json
Environment=LISTEN_PORT=5000
Environment=TRUSTED_PROXIES=127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
ExecStart=${BINARY_PATH}
Restart=always
User=root
Group=root

[Install]
WantedBy=multi-user.target
EOF

chmod 644 "/etc/systemd/system/${SERVICE_NAME}.service"

# Create example config if it doesn't exist
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
    "trusted_proxies": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOF
    chmod 640 "${CONFIG_DIR}/config.json"
fi

# Reload systemd
systemctl daemon-reload

print_message "$GREEN" "Installation completed! Version: ${VERSION}"
print_message "$GREEN" "To start the service:"
echo "systemctl start $SERVICE_NAME"
print_message "$GREEN" "To enable service on boot:"
echo "systemctl enable $SERVICE_NAME"
print_message "$YELLOW" "Remember to:"
echo "1. Update ${CONFIG_DIR}/config.json with your Firebase configuration"
echo "2. Copy your service account key to ${CONFIG_DIR}/service-account.json"
echo "3. Set appropriate permissions on configuration files"

exit 0 