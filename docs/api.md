# API Endpoints

All endpoints (except authentication) require Basic Authentication using the configured API key and secret.

## Authentication

### Get API Credentials
- **Endpoint**: `POST /api/method/notification_relay.api.auth.get_credential`
- **Description**: Get API credentials for a Frappe site
- **Body**: JSON with endpoint, protocol, port, token, and webhook_route
- **Authentication**: Not required for this endpoint

### Get Configuration
- **Endpoint**: `GET /api/method/notification_relay.api.get_config`
- **Description**: Returns VAPID public key and Firebase configuration
- **Authentication**: Not required for this endpoint

## Topic Management

### Subscribe to Topic
- **Endpoint**: `POST /api/method/notification_relay.api.topic.subscribe`
- **Description**: Subscribe a user to a notification topic
- **Query Parameters**:
  - `project_name`: Project identifier
  - `site_name`: Site name
  - `user_id`: User identifier
  - `topic_name`: Topic to subscribe to
- **Authentication**: Required

### Unsubscribe from Topic
- **Endpoint**: `POST /api/method/notification_relay.api.topic.unsubscribe`
- **Description**: Unsubscribe a user from a notification topic
- **Query Parameters**:
  - `project_name`: Project identifier
  - `site_name`: Site name
  - `user_id`: User identifier
  - `topic_name`: Topic to unsubscribe from
- **Authentication**: Required

## Token Management

### Add Token
- **Endpoint**: `POST /api/method/notification_relay.api.token.add`
- **Description**: Add a device token for a user
- **Query Parameters**:
  - `project_name`: Project identifier
  - `site_name`: Site name
  - `user_id`: User identifier
  - `fcm_token`: Firebase Cloud Messaging token
- **Authentication**: Required

### Remove Token
- **Endpoint**: `POST /api/method/notification_relay.api.token.remove`
- **Description**: Remove a device token for a user
- **Query Parameters**:
  - `project_name`: Project identifier
  - `site_name`: Site name
  - `user_id`: User identifier
  - `fcm_token`: Firebase Cloud Messaging token to remove
- **Authentication**: Required

## Notification Sending

### Send to User
- **Endpoint**: `POST /api/method/notification_relay.api.send_notification.user`
- **Description**: Send notification to a specific user
- **Query Parameters**:
  - `project_name`: Project identifier
  - `site_name`: Site name
  - `user_id`: User identifier
  - `title`: Notification title
  - `body`: Notification body
  - `data`: Additional data (optional)
- **Authentication**: Required

### Send to Topic
- **Endpoint**: `POST /api/method/notification_relay.api.send_notification.topic`
- **Description**: Send notification to a topic
- **Query Parameters**:
  - `topic_name`: Topic to send to
  - `title`: Notification title
  - `body`: Notification body
  - `data`: Additional data (optional)
- **Authentication**: Required 