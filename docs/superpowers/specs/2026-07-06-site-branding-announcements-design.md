# Site Branding And Announcements Design

## Goal

Let a site administrator customize the app store identity and publish a lightweight announcement without leaking device-side client state into the server.

## Scope

- Site title, icon URL, public domain, and generated source subscription URL.
- A single active announcement with level, title, body, optional link, and update timestamp.
- Public read access for the frontend and source clients.
- Site-admin write access through the existing settings surface.

## Data Model

The existing `site_settings` key/value table remains the storage source. New public keys are:

- `site_title`
- `site_icon_url`
- `site_public_url`
- `announcement_enabled`
- `announcement_level`
- `announcement_title`
- `announcement_body`
- `announcement_link_label`
- `announcement_link_url`
- `announcement_updated_at`

The server derives a `siteProfile` object from settings and environment fallback values. `site_public_url` is canonicalized by trimming trailing slashes and must be an HTTP(S) URL when present.

## API

- `GET /api/v1/site/profile` returns `{ site: SiteProfile }` without authentication.
- `GET/PATCH /api/v1/admin/settings` remains the site-admin write surface and accepts only allowlisted keys.
- `/source/v1/index.json` includes `site` and `announcement` metadata so standalone clients can display source identity and notices.

## UI

The server frontend loads `siteProfile` on startup. It uses the profile for:

- Sidebar brand title and icon.
- `document.title`.
- Home source subscription URL.
- A dismissible announcement banner and one-time toast notification when `announcement_updated_at` changes.

The Admin page separates settings into:

- Site identity.
- Announcement center.
- Policy settings.

Animations stay under 200ms and only use opacity, transform, border color, and background color transitions.

## Security

All setting writes are allowlisted and validated at the API boundary. React renders announcement text as escaped text, not HTML. Optional announcement links and icon URLs must be HTTP(S) URLs when present.
