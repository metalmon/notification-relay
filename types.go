package main

// Config represents the application configuration structure
type Config struct {
	Projects       map[string]ProjectConfig `json:"projects"`
	TrustedProxies string                   `json:"trusted_proxies,omitempty"`
	AllowedOrigins []string                 `json:"allowed_origins"`
}

// ProjectConfig represents project-specific Firebase configuration
type ProjectConfig struct {
	VapidPublicKey string         `json:"vapid_public_key"`
	FirebaseConfig FirebaseConfig `json:"firebase_config"`
	Exc            string         `json:"exc,omitempty"`
}

// ConfigResponse represents the response structure for the getConfig endpoint
type ConfigResponse struct {
	VapidPublicKey string                 `json:"vapid_public_key"`
	Config         map[string]interface{} `json:"config"`
	Exc            string                 `json:"exc,omitempty"`
}

// CredentialRequest represents a request for API credentials with validation requirements
type CredentialRequest struct {
	Endpoint     string `json:"endpoint"`      // Required: The endpoint URL
	Protocol     string `json:"protocol"`      // The protocol (http/https)
	Port         string `json:"port"`          // Optional: The port number
	Token        string `json:"token"`         // Required: Authentication token
	WebhookRoute string `json:"webhook_route"` // The webhook route path
}

// CredentialResponse represents an API credentials response
type CredentialResponse struct {
	Success     bool               `json:"success"`
	Message     string             `json:"message,omitempty"`
	Credentials *CredentialDetails `json:"credentials,omitempty"`
	Exc         string             `json:"exc,omitempty"`
}

// CredentialDetails contains generated API credentials
type CredentialDetails struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// Credentials represents a map of API credentials
type Credentials map[string]string

// Response represents the standard API response structure with optional fields
type Response struct {
	Message interface{} `json:"message,omitempty"` // Success message or status information
	Data    interface{} `json:"data,omitempty"`    // Response payload data
	Exc     string      `json:"exc,omitempty"`     // Error message for critical failures
}

// Decoration represents a notification title decoration rule for user notifications
type Decoration struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// TopicDecoration represents a notification title decoration rule for topic notifications
type TopicDecoration struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// NotificationPayload represents the structure of the notification payload
type NotificationPayload struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	ClickAction string            `json:"click_action,omitempty"`
}

// FirebaseConfig represents the Firebase configuration structure
type FirebaseConfig struct {
	ApiKey            string `json:"apiKey"`
	AuthDomain        string `json:"authDomain"`
	ProjectID         string `json:"projectId"`
	StorageBucket     string `json:"storageBucket"`
	MessagingSenderId string `json:"messagingSenderId"`
	AppId             string `json:"appId"`
}
