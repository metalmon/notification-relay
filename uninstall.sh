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

# Default paths
INSTALL_DIR="/opt/notification-relay"
CONFIG_DIR="/etc/notification-relay"
SERVICE_NAME="notification-relay"
BACKUP_DIR="/root/notification-relay-backup-$(date +%Y%m%d%H%M%S)"

# Parse command line options
KEEP_CONFIG=0
FORCE=0
REMOVE_CONTAINERS=1

while [[ $# -gt 0 ]]; do
    case $1 in
        --keep-config)
            KEEP_CONFIG=1
            shift
            ;;
        --force)
            FORCE=1
            shift
            ;;
        --no-remove-containers)
            REMOVE_CONTAINERS=0
            shift
            ;;
        *)
            print_message "$RED" "Unknown option: $1"
            echo "Usage: $0 [--keep-config] [--force] [--no-remove-containers]"
            exit 1
            ;;
    esac
done

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_message "$RED" "Please run as root"
    exit 1
fi

# Confirm uninstall
if [ $FORCE -eq 0 ]; then
    read -p "Are you sure you want to uninstall notification-relay? [y/N] " confirm
    if [[ ! "$confirm" =~ ^[yY]$ ]]; then
        print_message "$YELLOW" "Uninstall cancelled."
        exit 0
    fi
fi

# Stop and disable service
print_message "$YELLOW" "Stopping and disabling systemd service..."
systemctl stop $SERVICE_NAME 2>/dev/null
systemctl disable $SERVICE_NAME 2>/dev/null

# Backup configuration if requested
if [ $KEEP_CONFIG -eq 1 ] && [ -d "$CONFIG_DIR" ]; then
    print_message "$YELLOW" "Backing up configuration to $BACKUP_DIR..."
    mkdir -p "$BACKUP_DIR"
    cp -r "$CONFIG_DIR"/* "$BACKUP_DIR"/ 2>/dev/null
    print_message "$GREEN" "Configuration backed up to $BACKUP_DIR"
fi

# Remove Docker containers if they exist
if [ $REMOVE_CONTAINERS -eq 1 ]; then
    if [ -d "$INSTALL_DIR" ]; then
        print_message "$YELLOW" "Stopping and removing Docker containers..."
        if docker compose version >/dev/null 2>&1; then
            COMPOSE_CMD="docker compose"
        elif command -v docker-compose >/dev/null 2>&1; then
            COMPOSE_CMD="docker-compose"
        else
            COMPOSE_CMD=""
        fi
        
        if [ -n "$COMPOSE_CMD" ]; then
            cd "$INSTALL_DIR" && $COMPOSE_CMD -f docker-compose.yml -f docker-compose.prod.yml down -v 2>/dev/null
            # Remove any stray containers
            container_id=$(docker ps -a -q --filter name=notification-relay)
            if [ -n "$container_id" ]; then
                docker rm -f $container_id 2>/dev/null
            fi
        fi
    fi
fi

# Remove systemd service file
print_message "$YELLOW" "Removing systemd service file..."
rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload

# Remove installation directory
print_message "$YELLOW" "Removing installation directory..."
rm -rf "$INSTALL_DIR"

# Remove configuration directory if not keeping config
if [ $KEEP_CONFIG -eq 0 ]; then
    print_message "$YELLOW" "Removing configuration directory..."
    rm -rf "$CONFIG_DIR"
else
    print_message "$GREEN" "Configuration directory preserved at $CONFIG_DIR"
fi

# Check for binary installed directly (non-Docker installation)
if command -v notification-relay >/dev/null 2>&1; then
    print_message "$YELLOW" "Removing binary installation..."
    rm -f /usr/local/bin/notification-relay 2>/dev/null
    rm -f /usr/bin/notification-relay 2>/dev/null
fi

print_message "$GREEN" "Uninstallation completed!"
if [ $KEEP_CONFIG -eq 1 ]; then
    print_message "$GREEN" "Configuration files were backed up to $BACKUP_DIR"
fi

exit 0
