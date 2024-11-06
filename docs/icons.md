# Icons Configuration

The `icons.json` file maps projects to their notification icon paths. These icons are used as default icons for notifications when a specific icon is not provided.

## Structure

```json
{
    "project_site": "path/to/icon.png"
}
```

- `project_site`: The combined project and site name (e.g., "raven_erp-omniverse.com")
- Path value: Relative or absolute path to the icon file

## Example

```json
{
    "raven_erp-omniverse.com": "icons/raven.png",
    "hrms_erp-omniverse.com": "icons/hrms.png",
    "crm_erp-omniverse.com": "icons/crm.png"
}
```

## Icon Requirements

- Format: PNG recommended
- Size: 192x192 pixels recommended
- Location: Store icons in an `icons` directory in your project root
- Path: Use relative paths from the project root

## Usage

When sending a notification without a specific icon, the server will use the icon path defined for the project in this file. If no icon is defined for the project, or if the icon file doesn't exist, no icon will be shown in the notification. 