# Firebase and VAPID Setup Guide

This guide explains how to set up Firebase Cloud Messaging (FCM) and generate VAPID keys for the Notification Relay service.

## Firebase Setup

1. Create a Firebase Project:
   - Go to [Firebase Console](https://console.firebase.google.com/)
   - Click "Add project"
   - Enter a project name and follow the setup wizard

2. Add a Web App:
   - In your Firebase project console, click the gear icon ⚙️ and select "Project settings"
   - Under "Your apps", click the web icon (</>)
   - Register your app with a nickname (e.g., "notification-relay")
   - Firebase will provide your configuration object that looks like this:
   ```javascript
   {
     apiKey: "your-api-key",
     authDomain: "your-project.firebaseapp.com",
     projectId: "your-project-id",
     storageBucket: "your-project.appspot.com",
     messagingSenderId: "your-sender-id",
     appId: "your-app-id",
     measurementId: "your-measurement-id"
   }
   ```
   - Save this configuration - you'll need it for `config.json`

3. Generate Service Account Key:
   - In Project settings, go to the "Service accounts" tab
   - Click "Generate new private key"
   - Save the downloaded JSON file as `service-account.json`
   - Place this file in your Notification Relay configuration directory

4. Enable Cloud Messaging:
   - In Project settings, go to the "Cloud Messaging" tab
   - Note your Server key and Sender ID
   - Enable Cloud Messaging API if prompted

## VAPID Key Generation

VAPID (Voluntary Application Server Identification) keys are required for web push notifications. Here's how to generate them:

### Method 1: Using web-push Library

1. Install web-push globally:
   ```bash
   npm install -g web-push
   ```

2. Generate VAPID keys:
   ```bash
   web-push generate-vapid-keys
   ```

3. Save the output - you'll need both public and private keys:
   ```
   =======================================
   Public Key:
   BDd3_hVL9fZi9Ybo2UUzA284WG5FZR30_95YeZJsiApwXKpNcF1rRPF3foIiBHXRdJI2Qhumhf6_LFTeZaNndIo

   Private Key:
   MVRxVNbyqJ4_xn2Lgk6kdzJxJWjkcpCJrMMApuH-SVk
   =======================================
   ```

### Method 2: Using OpenSSL

1. Generate VAPID private key:
   ```bash
   openssl ecparam -name prime256v1 -genkey -noout -out vapid_private.pem
   ```

2. Generate VAPID public key:
   ```bash
   openssl ec -in vapid_private.pem -pubout -out vapid_public.pem
   ```

3. Convert keys to base64url format:
   ```bash
   # Private key
   openssl ec -in vapid_private.pem -outform DER | tail -c +8 | head -c 32 | base64 | tr -d '=' | tr '/+' '_-'

   # Public key
   openssl ec -in vapid_public.pem -pubin -outform DER | tail -c 65 | base64 | tr -d '=' | tr '/+' '_-'
   ```

## Configuration

1. Update your `config.json` with the Firebase configuration and VAPID public key:
   ```json
   {
       "projects": {
           "your-project": {
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
           }
       }
   }
   ```

2. Place your service account JSON file in the configuration directory:
   - Name it `service-account.json`
   - Ensure it has appropriate permissions (e.g., `600`)

## Security Considerations

1. Keep your service account key secure:
   - Never commit it to version control
   - Set appropriate file permissions
   - Consider using environment variables or secret management systems

2. Protect your VAPID private key:
   - Store it securely
   - Use it only on your server
   - Never expose it to clients

3. Firebase API key restrictions:
   - Set up API key restrictions in Firebase Console
   - Limit which domains can use your web API key
   - Consider using App Check for additional security

## Troubleshooting

1. Service Account Issues:
   - Ensure the service account has the "Firebase Admin SDK" role
   - Verify the file path is correct
   - Check file permissions

2. VAPID Key Issues:
   - Ensure keys are properly base64url encoded
   - Verify public key is correctly shared with clients
   - Check for any transcription errors

3. Firebase Configuration:
   - Verify all required fields are present
   - Check for typos in project IDs and keys
   - Ensure Cloud Messaging API is enabled

For more information, refer to:
- [Firebase Setup Documentation](https://firebase.google.com/docs/web/setup)
- [Web Push Documentation](https://developers.google.com/web/fundamentals/push-notifications)
- [Firebase Admin SDK Documentation](https://firebase.google.com/docs/admin/setup) 