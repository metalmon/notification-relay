#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status messages
print_status() {
    echo -e "${GREEN}[*]${NC} $1"
}

print_error() {
    echo -e "${RED}[!]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    print_error "Please run as root"
    exit 1
fi

# Create directories
print_status "Creating directories..."
mkdir -p /etc/notification-relay
mkdir -p /var/log/notification-relay

# Download and install binary
print_status "Downloading latest release..."
LATEST_RELEASE=$(curl -s https://api.github.com/repos/metalmon/relay-server/releases/latest | grep "browser_download_url.*tar.gz" | cut -d : -f 2,3 | tr -d \")

if [ -z "$LATEST_RELEASE" ]; then
    print_error "Failed to get latest release URL"
    exit 1
fi

wget -q $LATEST_RELEASE -O /tmp/notification-relay-linux-amd64.tar.gz

print_status "Installing binary..."
tar xzf /tmp/notification-relay-linux-amd64.tar.gz -C /tmp
mv /tmp/notification-relay-linux-amd64 /usr/local/bin/notification-relay
chmod +x /usr/local/bin/notification-relay
rm /tmp/notification-relay-linux-amd64.tar.gz

# Create systemd service
print_status "Creating systemd service..."
cat > /etc/systemd/system/notification-relay.service << EOL
[Unit]
Description=Frappe Push Notification Relay Server
After=network.target

[Service]
User=frappe
Group=www-data
WorkingDirectory=/etc/notification-relay
Environment="GOOGLE_APPLICATION_CREDENTIALS=/etc/notification-relay/service-account.json"
ExecStart=/usr/local/bin/notification-relay
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOL

# Create config file if it doesn't exist
if [ ! -f /etc/notification-relay/config.json ]; then
    print_status "Creating default config file..."
    cat > /etc/notification-relay/config.json << EOL
{
    "vapid_public_key": "",
    "firebase_config": {}
}
EOL
    print_warning "Please edit /etc/notification-relay/config.json with your configuration"
fi

# Create credentials file if it doesn't exist
if [ ! -f /etc/notification-relay/credentials.json ]; then
    print_status "Creating credentials file..."
    cat > /etc/notification-relay/credentials.json << EOL
{}
EOL
    chmod 600 /etc/notification-relay/credentials.json
fi

# Set proper permissions
print_status "Setting permissions..."
chown -R frappe:www-data /etc/notification-relay
chmod 750 /etc/notification-relay
chmod 640 /etc/notification-relay/config.json
chmod 600 /etc/notification-relay/credentials.json

# Reload systemd and enable service
print_status "Enabling service..."
systemctl daemon-reload
systemctl enable notification-relay

print_status "Installation complete!"
echo -e "${GREEN}Next steps:${NC}"
echo "1. Edit /etc/notification-relay/config.json with your configuration"
echo "2. Place your Firebase service account JSON file at /etc/notification-relay/service-account.json"
echo "3. Start the service with: systemctl start notification-relay"
echo "4. Check status with: systemctl status notification-relay" 