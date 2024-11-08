# Notification Decorations

This document describes the configuration of notification decorations.

## Configuration Files

### decoration.json
Used for user notifications. Contains patterns and templates for decorating notification titles based on project and site.

### topic-decoration.json
Used for topic notifications. Contains patterns and templates for decorating notification titles based on the topic name.

## Structure

### decoration.json
```json
{
    "project_site": {
        "notification_type": {
            "pattern": "regex_pattern",
            "template": "emoji_template {title}"
        }
    }
}
```

### topic-decoration.json
```json
{
    "topic_name": {
        "pattern": "regex_pattern",
        "template": "emoji_template {title}"
    }
}
```

## Parameters
- For `decoration.json`:
  - `project_site`: The combined project and site name (e.g., "raven_erp-omniverse.com")
  - `notification_type`: Category of notification (e.g., "new_message", "mention")
- For `topic-decoration.json`:
  - `topic_name`: The FCM topic name (e.g., "system_updates", "alerts")
- Common parameters:
  - `pattern`: Regular expression to match notification titles
  - `template`: Template string with emoji and {title} placeholder

## Examples

### decoration.json
```json
{
    "raven_erp-omniverse.com": {
        "new_message": {
            "pattern": ".*?(New message|Message received).*?",
            "template": "💬 {title}"
        },
        "mention": {
            "pattern": ".*?(mentioned you|tagged you).*?",
            "template": "👋 {title}"
        }
    }
}
```

### topic-decoration.json
```json
{
    "system_updates": {
        "pattern": ".*?(Update|Maintenance).*?",
        "template": "🔄 {title}"
    },
    "alerts": {
        "pattern": ".*?(Alert|Warning|Emergency).*?",
        "template": "🚨 {title}"
    },
    "announcements": {
        "pattern": ".*?(Announcement|Notice).*?",
        "template": "📢 {title}"
    }
}
```

## Usage
- For user notifications: The server checks the project_site and notification type to find matching patterns
- For topic notifications: The server checks the topic name to find matching patterns
- If a match is found, the title is formatted using the corresponding template
- If no match is found, the original title is used without decoration