# Decoration Configuration

The `decoration.json` file defines rules for decorating notifications with emojis and custom formatting based on the notification title patterns.

## Structure

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

- `project_site`: The combined project and site name (e.g., "raven_erp-omniverse.com")
- `notification_type`: Category of notification (e.g., "new_message", "mention")
- `pattern`: Regular expression to match notification titles
- `template`: Template string with emoji and {title} placeholder

## Example

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
        },
        "chat_invite": {
            "pattern": ".*?(invited you to chat|new chat invitation).*?",
            "template": "👥 {title}"
        }
    },
    "hrms_erp-omniverse.com": {
        "info": {
            "pattern": ".*?(Info|Notice|Note).*?",
            "template": "ℹ️ {title}"
        },
        "leave_request": {
            "pattern": ".*?(Leave Request|Time Off Request).*?",
            "template": "🗓️ {title}"
        },
        "timesheet": {
            "pattern": ".*?(Timesheet|Time Entry).*?",
            "template": "⏰ {title}"
        }
    }
}
```

## Usage

When a notification is sent, the server checks the title against the patterns defined for the project. If a match is found, the title is formatted using the corresponding template.

For example, if a notification with title "New message from John" is sent to raven_erp-omniverse.com, it will be decorated as "💬 New message from John". 