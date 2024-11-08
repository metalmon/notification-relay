# Configuration Guide

## Environment Variables

- `NOTIFICATION_RELAY_CONFIG`: Path to the configuration directory. If not set, the server will look for configuration files in the following locations in order:
  1. `./config.json`
  2. `/etc/notification-relay/config.json`

- `GOOGLE_APPLICATION_CREDENTIALS`: Path to Firebase service account JSON file. If not set, the server will look in:
  1. `./service-account.json`
  2. `/etc/notification-relay/service-account.json`

- `LISTEN_PORT`: Server port number. If not set, defaults to `5000`

## File Structure
The server uses several JSON configuration files:

1. `config.json` - Firebase and VAPID configuration
2. `credentials.json` - Generated API credentials for authenticated sites
3. `decoration.json` - Notification decoration rules
4. `icons.json` - Project icon paths
5. `user-device-map.json` - User device token mapping

## config.json

Main configuration file containing Firebase and VAPID settings:

```json
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
```

## credentials.json

This file stores the API credentials for authenticated sites. It is automatically managed by the server through the `/api/method/notification_relay.api.auth.get_credential` endpoint:

```json
{
    "generated_api_key_1": "generated_api_secret_1",
    "generated_api_key_2": "generated_api_secret_2"
}
```

## user-device-map.json

This file maintains the mapping between users and their FCM tokens:

```json
{
    "project_site": {
        "user_id": [
            "fcm_token_1",
            "fcm_token_2"
        ]
    }
}
```

Example:
```json
{
    "raven_erp-omniverse.com": {
        "user@example.com": [
            "fcm_token_123",
            "fcm_token_456"
        ]
    }
}
```

This file is automatically managed by the server - you don't need to edit it manually.

## File Locations

All configuration files should be placed in the project root directory:

```
/project_root
├── config.json
├── credentials.json
├── decoration.json
├── icons.json
├── user-device-map.json
└── icons/
    ├── raven.png
    ├── hrms.png
    └── crm.png
```

## .gitignore

# Configuration files containing sensitive data
config