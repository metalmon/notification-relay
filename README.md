# Push Notification Relay Server for Frappe Apps
This repo provides a push notification relay server for Frappe Apps such as Raven, implemented in Go.

## Installation

### Option 1: Easy Installation Script
Run the following command to automatically download and install everything:
```bash
curl -sSL https://raw.githubusercontent.com/metalmon/notification-relay/go/install.sh | sudo bash
```

After installation:
1. Edit `/etc/notification-relay/config.json` with your configuration
2. Place your Firebase service account JSON file at `/etc/notification-relay/service-account.json`
3. Start the service:
```bash
sudo systemctl start notification-relay
```

### Option 2: Quick Installation (Pre-compiled Binary)
1. Download the latest release:
```bash
wget https://github.com/metalmon/notification-relay/releases/latest/download/notification-relay-linux-amd64.tar.gz
```

2. Extract the binary:
```bash
tar xzf notification-relay-linux-amd64.tar.gz
sudo mv notification-relay-linux-amd64 /usr/local/bin/notification-relay
sudo chmod +x /usr/local/bin/notification-relay
```

### Option 3: Build from Source
If you prefer to compile the binary yourself:

1. Install Go (version 1.21 or higher)
2. Clone this repository:
```bash
git clone https://github.com/metalmon/notification-relay.git
cd notification-relay
```

3. Install dependencies and build:
```bash
go mod download
go build -o notification-relay
```

4. Install the binary:
```bash
sudo mv notification-relay /usr/local/bin/
sudo chmod +x /usr/local/bin/notification-relay
```

### Common Setup Steps
After installing the binary (either pre-compiled or self-built):

1. Create configuration directory:
```bash
sudo mkdir -p /etc/notification-relay
```

2. Create config.json (see Configuration section below)
```bash
sudo nano /etc/notification-relay/config.json
```

3. Set up systemd service:
```bash
sudo nano /etc/systemd/system/notification-relay.service
```

Add the following content:
```ini
[Unit]
Description=Frappe Push Notification Relay Server
After=network.target

[Service]
User=frappe
Group=www-data
WorkingDirectory=/etc/notification-relay
Environment="GOOGLE_APPLICATION_CREDENTIALS=/etc/notification-relay/service-account.json"
ExecStart=/usr/local/bin/notification-relay

[Install]
WantedBy=multi-user.target
```

4. Start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable notification-relay
sudo systemctl start notification-relay
```

5. Check status:
```bash
sudo systemctl status notification-relay
```

## Configuration
The server uses the following configuration files:

- `config.json`: Main configuration file containing VAPID key and Firebase config
- `credentials.json`: Stores API keys and secrets for authenticated sites
- `user-device-map.json`: Maps users to their device tokens
- [`decoration.json`](docs/decoration.md): Notification decoration rules and patterns
- [`icons.json`](docs/icons.md): Icon paths for different projects

For detailed configuration examples and structure, see the [configuration documentation](docs/configuration.md).

## API Endpoints

All endpoints (except authentication) require Basic Authentication using the configured API key and secret.

### Authentication
- `POST /api/method/notification_relay.api.auth.get_credential`
  - Get API credentials for a Frappe site
  - Body: JSON with endpoint, protocol, port, token, and webhook_route
  - No authentication required for this endpoint

- `GET /api/method/notification_relay.api.get_config`
  - Returns VAPID public key and Firebase configuration

- `POST /api/method/notification_relay.api.topic.subscribe`
  - Subscribe a user to a notification topic
  - Query params: project_name, site_name, user_id, topic_name

- `POST /api/method/notification_relay.api.topic.unsubscribe`
  - Unsubscribe a user from a notification topic
  - Query params: project_name, site_name, user_id, topic_name

- `POST /api/method/notification_relay.api.token.add`
  - Add a device token for a user
  - Query params: project_name, site_name, user_id, fcm_token

- `POST /api/method/notification_relay.api.token.remove`
  - Remove a device token for a user
  - Query params: project_name, site_name, user_id, fcm_token

- `POST /api/method/notification_relay.api.send_notification.user`
  - Send notification to a specific user
  - Query params: project_name, site_name, user_id, title, body, data

- `POST /api/method/notification_relay.api.send_notification.topic`
  - Send notification to a topic
  - Query params: topic_name, title, body, data

## ERPNext Integration
Add the `API_SECRET` & `API_KEY` in ERPNext Push Notification settings and enable the Push Notification Relay option.

## Docker Usage

### Quick Start with Docker Compose

1. Create required files and directories:
```bash
# Create configuration and log directories
mkdir -p ~/.notification-relay/logs
chmod 750 ~/.notification-relay
chmod 750 ~/.notification-relay/logs

# Create .env file
cat > .env << EOL
# Server configuration
PORT=5000
CONFIG_DIR=~/.notification-relay
LOG_DIR=~/.notification-relay/logs

# User configuration (for file permissions)
UID=$(id -u)
GID=$(id -g)
EOL

# Create config.json
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
    }
}
EOL

# Create other configuration files
touch ~/.notification-relay/credentials.json
touch ~/.notification-relay/user-device-map.json
touch ~/.notification-relay/decoration.json
touch ~/.notification-relay/icons.json

# Set proper permissions
chmod 600 ~/.notification-relay/*.json
```

2. Start the service:
```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f

# Rebuild after code changes
docker-compose up -d --build

# Stop service
docker-compose down
```

### Directory Structure
```
~/.notification-relay/
├── config.json
├── credentials.json
├── decoration.json
├── icons.json
├── user-device-map.json
└── logs/
    └── notification-relay.log
```

### Environment Variables

The following environment variables can be configured in `.env`:
- `PORT`: Server port number (default: 5000)
- `CONFIG_DIR`: Path to configuration directory (default: ~/.notification-relay)
- `LOG_DIR`: Path to log directory (default: ~/.notification-relay/logs)
- `UID`: User ID for file permissions (default: current user's UID)
- `GID`: Group ID for file permissions (default: current user's GID)

### Logs

The service logs are stored in `~/.notification-relay/logs/notification-relay.log`. You can view them in several ways:

```bash
# Using docker compose (all logs)
docker-compose logs -f

# Using docker compose (last 100 lines)
docker-compose logs --tail=100 -f

# Directly from log file
tail -f ~/.notification-relay/logs/notification-relay.log

# Last 100 lines from file
tail -n 100 ~/.notification-relay/logs/notification-relay.log
```

### Docker Compose Features
- Builds image locally - no need for Docker Hub
- Uses host user permissions for files
- Automatic service restart on failure
- Easy port configuration
- Proper volume mounting for configuration files
- Centralized logging with host directory mounting
