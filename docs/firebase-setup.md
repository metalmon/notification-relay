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

5. **Generate VAPID Keys**:
   - In the Firebase console, navigate to the **Cloud Messaging** tab under **Project settings**.
   - Scroll down to the **Web configuration** section.
   - In the **Web Push certificates** tab, click **Generate Key Pair**. The console will display the generated public key and private key.
   - Save the public key for use in your application and keep the private key secure.

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

For more information, refer to:
- [Firebase Setup Documentation](https://firebase.google.com/docs/web/setup)
- [Web Push Documentation](https://developers.google.com/web/fundamentals/push-notifications)
- [Firebase Admin SDK Documentation](https://firebase.google.com/docs/admin/setup)