# Push Notification Relay Server for Frappe Apps
This repo provides a push notification relay server for Frappe Apps such as Raven, implemented in Go.

## Installation

### Option 1: Docker Compose (Recommended)
#### 1. Create .env file
Copy the example environment file and adjust the values as needed:
```bash
# Copy the example file
cp .env.example .env

# Edit the file with your settings
nano .env
```

The following environment variables can be configured in `.env`:
- `PORT`: Server port number (default: 5000)
- `CONFIG_DIR`: Path to configuration directory (default: ~/.notification-relay)
- `LOG_DIR`: Path to log directory (default: ~/.notification-relay/logs)
- `UID`: User ID for file permissions (default: current user's UID)
- `GID`: Group ID for file permissions (default: current user's GID)
- `TRUSTED_PROXIES`: Comma-separated list of trusted proxy CIDR ranges. Special values:
  - `*`: Trust all proxies (not recommended)
  - `none`: Trust no proxies
  - Default: `127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`

To get your current user's UID and GID:
```bash
echo "UID=$(id -u)"
echo "GID=$(id -g)"
```


#### 2. Create required files and directories:

Create configuration and log directories
```bash
mkdir -p ~/.notification-relay/logs
chmod 750 ~/.notification-relay
chmod 750 ~/.notification-relay/logs
```

Place your Firebase service account JSON file at ~/.notification-relay/service-account.json

Create config.json
```bash
cat > ~/.notification-relay/config.json << EOL
{
    "vapid_public_key": "your_vapid_public_key",
    "firebase_config": {
        "apiKey": "your-firebase-api-key",
        "authDomain": "your-project.firebaseapp.com",
        "projectId": "your-project-id",
        "storageBucket": "your-project.appspot.com",
        "messagingSenderId": "your-sender-id",
        "appId": "your-app-id",
        "measurementId": "your-measurement-id"
    },
    "trusted_proxies": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
EOL
```

Create other configuration files and set proper permissions
```bash
touch ~/.notification-relay/credentials.json
touch ~/.notification-relay/user-device-map.json
touch ~/.notification-relay/decoration.json
touch ~/.notification-relay/topic-decoration.json
touch ~/.notification-relay/icons.json

# Set proper permissions
chmod 600 ~/.notification-relay/*.json
```

#### 3. Start the service:
```bash
# Build and start
docker-compose up -d
```
```bash
# View logs
docker-compose logs -f
```
```bash
# Rebuild after code changes
docker-compose up -d --build
```
```bash
# Stop service
docker-compose down
```

### Option 2: Docker Build
Build the image
```bash
# Build the image
docker build -t notification-relay .
```

Run the container
```bash
docker run -d \
  -p 5000:5000 \
  -v ~/.notification-relay:/etc/notification-relay \
  -v ~/.notification-relay/logs:/var/log/notification-relay \
  --user $(id -u):$(id -g) \
  notification-relay
```

### Option 3: Easy Installation Script
Run the following command to automatically download and install everything:
```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/main/install.sh | sudo bash
```

After installation:
1. Edit `/etc/notification-relay/config.json` with your configuration
2. Place your Firebase service account JSON file at `/etc/notification-relay/service-account.json`
3. Start the service:
```bash
sudo systemctl start notification-relay
```

## Configuration
The server uses the following configuration files:

### Service Account
Firebase service account JSON file can be configured in several ways:
1. Via environment variable: `GOOGLE_APPLICATION_CREDENTIALS`
2. Default locations (checked in order):
   - `./service-account.json`
   - `/etc/notification-relay/service-account.json`

### Other Configuration Files
- `config.json`: Main configuration file containing VAPID key, Firebase config and trusted proxies
- `credentials.json`: Stores API keys and secrets for authenticated sites
- `user-device-map.json`: Maps users to their device tokens
- [`decoration.json`](docs/decoration.md): Notification decoration rules and patterns for user notifications
- [`topic-decoration.json`](docs/decoration.md): Notification decoration rules and patterns for topic notifications
- [`icons.json`](docs/icons.md): Icon paths for different projects

For detailed configuration examples and structure, see the [configuration documentation](docs/configuration.md).

## API Documentation

The server provides several endpoints for managing notifications, topics, and device tokens. For detailed API documentation, see [API Documentation](docs/api.md).

Key features include:
- Authentication and credential management
- Topic subscription management
- Device token management
- User and topic notification sending

## Frappe Integration
Add push relay server url to your site configuration
```bash
# Change <your site> and <your_push_relay_url:port> for according values
bench --site <your site> set-config push_relay_server_url "<your_push_relay_url:port>"
```
Enable the Push Notification Relay option in your app.

#### Logs

The service logs are stored in `~/.notification-relay/logs/notification-relay.log`. You can view them in several ways:

```bash
# Using docker compose (all logs)
docker-compose logs -f
```
```bash
# Using docker compose (last 100 lines)
docker-compose logs --tail=100 -f
```
```bash
# Directly from log file
tail -f ~/.notification-relay/logs/notification-relay.log
```
```bash
# Last 100 lines from file
tail -n 100 ~/.notification-relay/logs/notification-relay.log
```


