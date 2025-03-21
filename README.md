# Notification Relay Service

A service for managing web push notifications across multiple projects using Firebase Cloud Messaging (FCM).

## Features

- Multi-project support with separate Firebase configurations
- Topic-based notifications
- User-specific notifications
- Customizable notification decorations
- Icon management
- Secure API authentication
- Docker support

## Installation Methods

### Easy Install (Binary)
Installs the latest release binary as a system service:

```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/main/install-binary.sh | sudo bash
```

After installation:
1. Configure your Firebase settings:
   ```bash
   sudo nano /etc/notification-relay/config.json
   ```

2. Add your service account key:
   ```bash
   sudo cp path/to/service-account.json /etc/notification-relay/
   ```

3. Start the service:
   ```bash
   sudo systemctl start notification-relay
   sudo systemctl enable notification-relay
   ```

### Production Install (Docker + System Service)
Full production installation with Docker support:

```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/main/install.sh | sudo bash
```

This method:
- Sets up Docker containers
- Creates system service
- Configures logging
- Sets up proper permissions
- Provides production-ready deployment

## Documentation

- [Configuration Guide](docs/configuration.md) - Detailed configuration instructions
- [Firebase Setup Guide](docs/firebase-setup.md) - How to set up Firebase and generate VAPID keys
- [API Documentation](docs/api.md) - API endpoints and usage
- [Decoration Guide](docs/decoration.md) - Notification decoration configuration
- [Icons Guide](docs/icons.md) - Icon configuration and usage

## Configuration Files

- `config.json` - Main configuration file (required)
- `credentials.json` - API credentials (auto-generated)
- `decoration.json` - Notification decoration rules
- `topic-decoration.json` - Topic-specific decoration rules
- `icons.json` - Project icon paths
- `user-device-map.json` - User device token mapping (auto-generated)

## Environment Variables

- `NOTIFICATION_RELAY_CONFIG` - Path to config.json
- `GOOGLE_APPLICATION_CREDENTIALS` - Path to service account JSON
- `LISTEN_PORT` - Server port (default: 5000)
- `TRUSTED_PROXIES` - Trusted proxy CIDR ranges

## Security Considerations

1. Always use HTTPS in production
2. Configure trusted proxies appropriately
3. Keep service account and VAPID keys secure
4. Use strong API credentials
5. Set appropriate file permissions

## Uninstallation

To uninstall the Notification Relay Service:

```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/main/uninstall.sh | sudo bash
```

### Uninstallation Options

The uninstall script supports several options:

- `--keep-config` - Preserves configuration files by creating a backup before removal
- `--force` - Uninstalls without asking for confirmation
- `--no-remove-containers` - Leaves Docker containers intact (useful if they're managed separately)

Examples:

```bash
# Standard uninstall with confirmation
sudo ./uninstall.sh

# Uninstall but keep configuration files backed up
sudo ./uninstall.sh --keep-config

# Force uninstall without confirmation
sudo ./uninstall.sh --force
```

### Manual Cleanup

If you need to manually clean up:

1. Stop and disable the service:
   ```bash
   sudo systemctl stop notification-relay
   sudo systemctl disable notification-relay
   ```

2. Remove Docker containers (if using Docker):
   ```bash
   cd /opt/notification-relay
   sudo docker compose down -v
   ```

3. Remove installation files:
   ```bash
   sudo rm -rf /opt/notification-relay
   sudo rm -rf /etc/notification-relay
   sudo rm /etc/systemd/system/notification-relay.service
   sudo systemctl daemon-reload
   ```

## License

[MIT License](LICENSE)


