# Push Notification Relay Server for Frappe Apps
This repo provides a push notification relay server for Frappe Apps such as Raven, implemented in Go.

## Getting Started
To run this application, follow these steps:

1. Clone this project
2. Install Go (version 1.21 or higher) if not already installed
3. Install dependencies:
```bash
go mod download
```

4. Create a Firebase Project & get Service Account credentials [Link](https://sharma-vikashkr.medium.com/firebase-how-to-setup-a-firebase-service-account-836a70bb6646)

5. Follow **Register your app** under Step 1 in the [Firebase documentation](https://firebase.google.com/docs/web/setup#register-app) and obtain the `FIREBASE_CONFIG` JSON object.

6. Follow this StackOverflow [Link](https://stackoverflow.com/a/54996207) to generate a VAPID key.

7. Create a `config.json` file in the project root with the following structure:
```json
{
    "vapid_public_key": "your_vapid_public_key",
    "firebase_config": {
        "apiKey": "your-api-key",
        "authDomain": "your-project.firebaseapp.com",
        "projectId": "your-project-id",
        "storageBucket": "your-project.appspot.com",
        "messagingSenderId": "your-sender-id",
        "appId": "your-app-id",
        "measurementId": "your-measurement-id"
    },
    "api_key": "your-api-key",
    "api_secret": "your-api-secret"
}
```

8. Build and run the application:
```bash
go build
./notification-relay
```

## Running as a Systemd Service
Create a systemd service file at `/etc/systemd/system/push-relay.service`:

```ini
[Unit]
Description=Frappe Push Notification Relay Server
After=network.target

[Service]
User=frappe
Group=www-data
WorkingDirectory=/home/frappe/relay-server
Environment="GOOGLE_APPLICATION_CREDENTIALS=/home/frappe/relay-server/service-account.json"
ExecStart=/home/frappe/relay-server/notification-relay

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable push-relay
sudo systemctl start push-relay
```

## Configuration
The server uses the following configuration files:

- `config.json`: Main configuration file containing API keys and Firebase config
- `user-device-map.json`: Maps users to their device tokens
- [`decoration.json`](docs/decoration.md): Notification decoration rules and patterns
- [`icons.json`](docs/icons.md): Icon paths for different projects

For detailed configuration examples and structure, see the [configuration documentation](docs/configuration.md).

## API Endpoints

All endpoints require Basic Authentication using the configured API key and secret.

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
