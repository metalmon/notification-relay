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

## Quick Install (Linux)

### Option 1: One-line Install
```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/main/install-binary.sh | sudo bash
```

### Option 2: Manual Install
1. Download the install script:
   ```bash
   curl https://raw.githubusercontent.com/metalmon/notification-relay/main/install-binary.sh
   chmod +x install-binary.sh
   ```

2. Run the installer:
   ```bash
   sudo ./install-binary.sh
   ```

3. Configure and start:
  ```bash
   # Update configuration
   sudo nano /etc/notification-relay/config.json
  ```
  ```bash
   # Add your service account key
   sudo cp path/to/service-account.json /etc/notification-relay/
  ```
  ```bash
   # Start and enable the service
   sudo systemctl start notification-relay
   sudo systemctl enable notification-relay
   ```

## Production Deployment with Docker

For production environments, we recommend using Docker:

1. Create configuration directory:
   ```bash
   sudo mkdir -p /etc/notification-relay
   ```

2. Set up configuration:
   ```bash
   # Copy your config files
   sudo cp config.json /etc/notification-relay/
   sudo cp service-account.json /etc/notification-relay/
   ```

3. Configure environment variables:
   ```bash
   # Copy example environment file
   cp .env.example .env

   # Edit environment variables as needed
   nano .env
   ```

4. Run with Docker Compose:
   ```bash
   docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
   ```

### Environment Variables

You can configure the service using environment variables either through the `.env` file (for Docker) or system environment:

- `NOTIFICATION_RELAY_CONFIG` - Path to config.json (default: /etc/notification-relay/config.json)
- `GOOGLE_APPLICATION_CREDENTIALS` - Path to service account JSON (default: /etc/notification-relay/service-account.json)
- `LISTEN_PORT` - Server port (default: 5000)
- `TRUSTED_PROXIES` - Trusted proxy CIDR ranges
- `DOCKER_UID` - User ID for Docker container (default: 1000)
- `DOCKER_GID` - Group ID for Docker container (default: 1000)

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

## License

[MIT License](LICENSE)


