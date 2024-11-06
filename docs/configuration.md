# Configuration Guide

## File Structure
The server uses several JSON configuration files:

1. `config.json` - Main configuration file
2. `decoration.json` - Notification decoration rules
3. `icons.json` - Project icon paths
4. `user-device-map.json` - User device token mapping
5. `credentials.json` - API credentials for authenticated sites

## credentials.json

This file stores the API credentials for authenticated sites:

```json
{
    "api_key_1": "api_secret_1",
    "api_key_2": "api_secret_2"
}
```

This file is automatically managed by the server - you don't need to edit it manually.

## config.json

Main configuration file containing API keys and Firebase settings:

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