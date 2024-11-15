#!/bin/bash
set -e

# Set default environment variables
export LISTEN_PORT=${LISTEN_PORT:-5000}
export NOTIFICATION_RELAY_CONFIG=${NOTIFICATION_RELAY_CONFIG:-/etc/notification-relay/config.json}
export GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-/etc/notification-relay/service-account.json}

# Log configuration
echo "Starting notification-relay server..."
echo "Port: $LISTEN_PORT"
echo "Config path: $NOTIFICATION_RELAY_CONFIG"
echo "Service account path: $GOOGLE_APPLICATION_CREDENTIALS"

# Run the server
exec ./notification-relay 