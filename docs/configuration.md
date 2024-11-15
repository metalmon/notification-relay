# Configuration Guide

## Environment Variables

- `NOTIFICATION_RELAY_CONFIG`: Path to the configuration directory. If not set, the server will look for configuration files in the following locations in order:
  1. `./config.json`
  2. `/etc/notification-relay/config.json`

- `GOOGLE_APPLICATION_CREDENTIALS`: Path to Firebase service account JSON file. If not set, the server will look in:
  1. `./service-account.json`
  2. `/etc/notification-relay/service-account.json`

- `LISTEN_PORT`: Server port number. If not set, defaults to `5000`

- `TRUSTED_PROXIES`: Comma-separated list of trusted proxy CIDR ranges. If not set, uses value from config.json. Special values:
  - `*`: Trust all proxies (not recommended)
  - `none`: Trust no proxies
  - Default: `127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`

## Proxy Configuration

The server requires trusted proxy configuration for proper handling of client IP addresses behind reverse proxies. This can be set in two ways:

1. Through environment variable `TRUSTED_PROXIES`
2. In `config.json` using the `trusted_proxies` field

The configuration accepts:
- CIDR ranges (e.g., "10.0.0.0/8")
- Multiple ranges separated by commas
- Special values: "*" (trust all) or "none" (trust none)

Example CIDR configurations:
- Local proxy: `127.0.0.1/32`
- Private networks: `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`
- Cloud provider: `35.190.247.0/24`

## File Structure
The server uses several JSON configuration files:

1. `config.json` - Project-specific Firebase and VAPID configurations (required)
2. `credentials.json` - Generated API credentials for authenticated sites
3. `decoration.json` - Notification decoration rules and patterns for user notifications
4. `topic-decoration.json` - Notification decoration rules and patterns for topic notifications
5. `icons.json` - Project icon paths
6. `user-device-map.json` - User device token mapping

## config.json
Main configuration file containing project-specific Firebase and VAPID settings. The `trusted_proxies` field is required:

```json
{
    "projects": {
        "project1": {
            "vapid_public_key": "your-vapid-public-key",
            "firebase_config": {
                "apiKey": "your-firebase-api-key",
                "authDomain": "your-project.firebaseapp.com",
                "projectId": "your-project-id",
                "storageBucket": "your-project.appspot.com",
                "messagingSenderId": "your-sender-id",
                "appId": "your-app-id",
                "measurementId": "your-measurement-id"
            }
        },
        "project2": {
            "vapid_public_key": "project2_vapid_public_key",
            "firebase_config": {
                "apiKey": "project2-firebase-api-key",
                "authDomain": "project2.firebaseapp.com",
                "projectId": "project2-id",
                "storageBucket": "project2.appspot.com",
                "messagingSenderId": "project2-sender-id",
                "appId": "project2-app-id",
                "measurementId": "project2-measurement-id"
            }
        }
    },
    "trusted_proxies": "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
}
```

## Security Considerations

1. Keep your service account key secure:
   - Never commit it to version control
   - Set appropriate file permissions
   - Consider using environment variables or secret management systems

2. Firebase API key restrictions:
   - Set up API key restrictions in Firebase Console
   - Limit which domains can use your web API key
   - Consider using App Check for additional security

For more information, refer to:
- [Firebase Setup Documentation](https://firebase.google.com/docs/web/setup)
- [Web Push Documentation](https://developers.google.com/web/fundamentals/push-notifications)
- [Firebase Admin SDK Documentation](https://firebase.google.com/docs/admin/setup)

## decoration.json
This file defines notification title decoration rules for user notifications. Rules are applied based on project and pattern matching:

```json
{
    "project1_example.com": {
        "error": {
            "pattern": "^Error:",
            "template": "⚠️ {title}"
        },
        "success": {
            "pattern": "^Success:",
            "template": "✅ {title}"
        }
    }
}
```

## topic-decoration.json
This file defines notification title decoration rules for topic notifications:

```json
{
    "announcements": {
        "pattern": ".*",
        "template": "📢 {title}"
    },
    "alerts": {
        "pattern": "^Alert:",
        "template": "🚨 {title}"
    }
}
```

## icons.json
This file maps projects to their notification icon paths:

```json
{
    "project1_example.com": "/static/icons/project1-icon.png",
    "project2_example.com": "/static/icons/project2-icon.png"
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
    "project1_example.com": {
        "user@example.com": [
            "fcm_token_123",
            "fcm_token_456"
        ]
    }
}
```

This file is automatically managed by the server - you don't need to edit it manually.

## Notification Decoration
The server supports two types of notification decorations:

1. User Notifications (`decoration.json`):
   - Applied based on project and pattern matching
   - Useful for adding icons to specific types of notifications
   - Patterns are matched against notification titles
   - Templates can include the original title using `{title}`

2. Topic Notifications (`topic-decoration.json`):
   - Applied based on topic name and pattern matching
   - Useful for adding topic-specific prefixes or icons
   - Patterns are matched against notification titles
   - Templates can include the original title using `{title}`

## Icons
Project icons are automatically added to notifications based on the project key (project_name + site_name). For topic notifications, icons can be specified in the notification data using the `icon` field.

## Project Keys
Throughout the configuration, project keys are formatted as `project_name_site_name`. For example:
- Project name: "project1"
- Site name: "example.com"
- Project key: "project1_example.com"

This key format is used in:
- User device mapping
- Decoration rules
- Icon paths
