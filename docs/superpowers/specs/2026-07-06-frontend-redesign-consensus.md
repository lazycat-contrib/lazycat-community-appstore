# Frontend Redesign Consensus

## Goal

Redesign the server and standalone client frontends as three clear product surfaces:

- **Server storefront:** public app discovery, categories, app detail, source subscription instructions, and a quiet login entry.
- **Server backstage:** authenticated maintainer and site-admin operations, split by task.
- **Standalone client:** source-first app browsing, source management, installed inventory, updates, install history, and rollback.

The redesign prioritizes visual quality, app-store clarity, useful motion, clear operation feedback, ease of use, and stability, in that order.

## Product Boundaries

### Server Storefront

The public server UI is a marketplace, not a dashboard. It should answer:

- What apps are available?
- Which categories can I browse?
- Which apps are featured or recently updated?
- How do I subscribe to this store from the client?
- Where do I log in if I maintain apps or manage the site?

It must not show device-installed app lists, client source health, admin metrics, submit forms, user-management details, or review queues.

### Server Backstage

The authenticated server UI is an operations console. It should answer:

- What apps do I own and what needs my attention?
- How do I submit or update an app?
- Which comments, review states, and notifications require action?
- How does a site admin manage reviews, users, taxonomy, collections, storage, email, source policy, and site branding?

It should be denser than the public storefront, but each tab must still have one job. Forms should open from explicit buttons or scoped panels instead of occupying the default view.

### Standalone Client

The standalone client is a local installer. It should answer:

- Which sources do I have?
- Which apps came from which source?
- Which categories are available inside the selected source?
- Which installed apps can be updated?
- What did I install, when, and from which source?
- Which older source versions can I reinstall or roll back to?

It must not show server review/admin concepts or pretend that the server knows local device inventory. Unknown existing device apps should be shown as local apps with unknown source attribution.

## Information Architecture

### Server Storefront Navigation

- Home
- Apps
- Categories
- Collections
- Source Instructions
- Login

Home is the first screen. It uses a store-like layout: search, category rail, featured/recent sections, and a source subscription panel. The app list is its own page or tab, not a block buried inside an overview.

### Server Backstage Navigation

- My Apps
  - Overview
  - Apps
  - Submit
  - Comments
  - Tokens
  - Groups
- Admin
  - Reviews
  - Site
  - Taxonomy
  - Collections
  - Users
  - Storage
  - Email
  - Audit or activity

Admin tools are only available to users with permission. My Apps is for maintainers; Admin is for governance. They should not be merged into one long profile page.

### Standalone Client Navigation

- Discover
- Updates
- Installed
- History
- Sources

Discover is the source catalog. Updates appears as a peer category when updateable apps exist; if no updates exist, the category and bulk-update action are hidden. Sources is complete but visually quieter than Discover and Installed.

## Design DNA

```json
{
  "meta": {
    "name": "LazyCat Private Store Frontend",
    "description": "A polished private app-store interface for public browsing, admin operations, and local source-based installation.",
    "source_references": ["Apple App Store", "Google Play", "NAS package centers", "Astryx neutral components"],
    "created_at": "2026-07-06"
  },
  "design_system": {
    "color": {
      "palette_type": "analogous with cool accent",
      "primary": { "hex": "#0F6B4D", "role": "brand actions and active navigation" },
      "secondary": { "hex": "#225C9B", "role": "links, info, and source metadata" },
      "accent": { "hex": "#B7781B", "role": "featured/update/status emphasis used sparingly" },
      "neutral": {
        "scale": ["#FBFCF8", "#F3F6F1", "#D5DDD4", "#63706A", "#1D2421"],
        "usage": "warm neutral surfaces with high-contrast text"
      },
      "semantic": {
        "success": "#0F6B4D",
        "warning": "#B7781B",
        "error": "#B83333",
        "info": "#225C9B"
      },
      "surface": {
        "background": "#EDF0EC",
        "card": "#FFFFFF",
        "elevated": "#FBFCF8"
      },
      "contrast_strategy": "dark-on-light dominant with independent dark theme tokens"
    },
    "typography": {
      "type_scale": {
        "display": { "size": "40px", "weight": "700", "line_height": "1.08", "tracking": "0" },
        "heading_1": { "size": "32px", "weight": "700", "line_height": "1.15", "tracking": "0" },
        "heading_2": { "size": "24px", "weight": "650", "line_height": "1.22", "tracking": "0" },
        "heading_3": { "size": "18px", "weight": "650", "line_height": "1.3", "tracking": "0" },
        "body": { "size": "16px", "weight": "400", "line_height": "1.55", "tracking": "0" },
        "body_small": { "size": "14px", "weight": "400", "line_height": "1.45", "tracking": "0" },
        "caption": { "size": "12px", "weight": "500", "line_height": "1.35", "tracking": "0" },
        "overline": { "size": "12px", "weight": "650", "line_height": "1.25", "tracking": "0" }
      },
      "font_families": {
        "heading": "Inter, system-ui, sans-serif",
        "body": "Inter, system-ui, sans-serif",
        "mono": "ui-monospace, SFMono-Regular, Menlo, monospace"
      },
      "font_style_notes": "System-first UI typography with tabular numerals for versions, counts, and dates."
    },
    "spacing": {
      "base_unit": "4px",
      "scale": ["4px", "8px", "12px", "16px", "24px", "32px", "48px", "64px"],
      "content_density": "storefront comfortable, backstage compact, client balanced",
      "section_rhythm": "clear page bands; repeated cards use 16-24px gaps"
    },
    "layout": {
      "grid_system": "responsive CSS grid and flex; no horizontal overflow at 320px",
      "max_content_width": "1200px storefront, 1440px admin console",
      "columns": "1 mobile, 2 tablet, 3-4 desktop for app cards; admin uses sidebar plus table/list content",
      "gutter": "16px mobile, 24px tablet, 32px desktop",
      "breakpoints": ["320px", "375px", "768px", "1024px", "1440px"],
      "alignment_tendency": "strict grid with clear leading edges"
    },
    "shape": {
      "border_radius": { "small": "6px", "medium": "8px", "large": "12px", "pill": "999px" },
      "border_usage": "subtle 1px borders on fields, cards, tables, and sheets",
      "divider_style": "low-contrast tokenized dividers"
    },
    "elevation": {
      "shadow_style": "soft diffused only for modal/sheet elevation",
      "levels": {
        "low": "0 1px 2px rgba(0,0,0,.04)",
        "medium": "0 12px 32px rgba(0,0,0,.12)",
        "high": "0 24px 80px rgba(0,0,0,.22)"
      },
      "depth_cues": "surface contrast, border, and small shadows"
    },
    "iconography": {
      "style": "single-color outline",
      "stroke_weight": "1.75-2px",
      "size_scale": ["16px", "18px", "20px", "24px"],
      "preferred_set": "lucide-react"
    },
    "motion": {
      "easing": "var(--ease-out) for entry and feedback, var(--ease-in-out) for layout movement, var(--ease-drawer) for sheets",
      "duration_scale": { "micro": "120ms", "normal": "180ms", "macro": "240ms" },
      "entrance_pattern": "opacity plus 4-8px translate; dialogs scale from 0.97, never 0",
      "exit_pattern": "shorter opacity/translate exit",
      "philosophy": "functional, responsive, and reduced-motion aware"
    },
    "components": {
      "button_style": "Astryx buttons for command actions, icon buttons for toolbar actions",
      "input_style": "Astryx TextInput/TextArea/Selector for forms; no nested native inputs inside styled inputs",
      "card_style": "8px-radius product cards and compact operational rows",
      "navigation_pattern": "public top navigation; admin/client adaptive sidebar or top tabs; mobile bottom/top tabs as appropriate",
      "modal_style": "origin-aware lightweight dialogs for add/edit/submit flows",
      "list_style": "product cards for browsing, compact rows for operations and installed inventory",
      "component_notes": "Page containers own data and routing; components own presentation and local form state only."
    }
  },
  "design_style": {
    "aesthetic": {
      "mood": ["clear", "trustworthy", "polished", "quiet", "capable"],
      "visual_metaphor": "private app marketplace plus NAS package center",
      "era_influence": "modern platform app stores",
      "genre": "consumer marketplace plus admin console",
      "personality_traits": ["organized", "practical", "careful", "confident"],
      "adjectives": ["content-first", "calm", "precise", "responsive"]
    },
    "visual_language": {
      "complexity": "moderate",
      "ornamentation": "subtle accents only",
      "whitespace_usage": "comfortable on public/client browsing, efficient in admin tables",
      "visual_weight_distribution": "strong identity/header then scannable lists",
      "focal_strategy": "single primary workflow per screen",
      "contrast_level": "medium-high",
      "texture_usage": "none"
    },
    "composition": {
      "hierarchy_method": "typographic scale, spacing, and action placement",
      "balance_type": "structured asymmetric",
      "flow_direction": "top-to-bottom with filters before results",
      "grouping_strategy": "task tabs and scoped panels",
      "negative_space_role": "separate workflows and reduce dashboard clutter"
    },
    "imagery": {
      "photo_treatment": "real app icons and screenshots when available",
      "illustration_style": "none for primary product surfaces",
      "graphic_elements": "category chips, source badges, version/status badges",
      "pattern_usage": "none",
      "image_shape": "8px icon and screenshot containers"
    },
    "interaction_feel": {
      "feedback_style": "button press, loading state, inline validation, toast confirmation",
      "hover_behavior": "subtle background/border changes gated by hover media queries",
      "transition_personality": "snappy and stable",
      "loading_style": "skeleton for lists, spinner only inside buttons",
      "microinteraction_density": "moderate"
    },
    "brand_voice_in_ui": {
      "tone": "direct and helpful",
      "formality": "professional",
      "cta_style": "clear verbs",
      "empty_state_approach": "one next action",
      "error_tone": "state cause and recovery"
    }
  },
  "visual_effects": {
    "overview": {
      "effect_intensity": "subtle-accent",
      "performance_tier": "lightweight",
      "fallback_strategy": "reduced motion disables movement and keeps opacity/color feedback",
      "primary_technology": "CSS transitions"
    },
    "background_effects": {
      "type": "none",
      "description": "No decorative orb, bokeh, or animated gradient backgrounds.",
      "technology": "CSS tokens",
      "params": { "color_palette": "surface tokens", "speed": "none", "density": "none", "opacity": "none", "blend_mode": "normal" }
    },
    "particle_systems": { "enabled": false, "type": "none", "description": "Not used.", "technology": "none", "params": { "count": 0, "shape": "none", "size_range": "none", "movement_pattern": "none", "color_behavior": "none", "interaction": "none", "spawn_area": "none" } },
    "3d_elements": { "enabled": false, "type": "none", "description": "Not used.", "technology": "none", "params": { "renderer": "none", "lighting": "none", "camera": "none", "materials": "none", "geometry": "none", "post_processing": [], "interaction_model": "none" } },
    "shader_effects": { "enabled": false, "type": "none", "description": "Not used.", "technology": "none", "params": { "uniforms": {}, "vertex_manipulation": "none", "fragment_output": "none", "noise_type": "none", "distortion": "none" } },
    "scroll_effects": {
      "parallax": { "enabled": false, "layers": 0, "depth_range": "none", "speed_curve": "none" },
      "scroll_triggered_animations": { "enabled": true, "trigger_points": "initial list load only", "animation_type": "short fade-up stagger", "scrub_behavior": "none" },
      "scroll_morphing": { "enabled": false, "description": "Not used." }
    },
    "text_effects": { "type": "none", "description": "Text remains readable and stable.", "technology": "CSS", "params": { "split_strategy": "none", "animation_per_unit": "none", "stagger": "none", "effect_style": "none" } },
    "cursor_effects": { "enabled": false, "type": "none", "description": "Native cursor behavior.", "params": { "shape": "native", "size": "native", "blend_mode": "normal", "trail": "none", "interaction_zone": "none" } },
    "image_effects": { "type": "reveal-clip", "description": "Optional image/screenshot reveal when entering viewport.", "technology": "CSS", "params": { "filter_pipeline": "none", "hover_transform": "subtle scale under hover media only", "reveal_animation": "opacity + translate", "distortion_type": "none" } },
    "glassmorphism_neumorphism": { "enabled": false, "style": "none", "params": { "blur_radius": "0", "transparency": "0", "border_treatment": "token border", "shadow_type": "none", "light_source_angle": "none" } },
    "canvas_drawings": { "enabled": false, "type": "none", "description": "Not used.", "technology": "none", "params": { "draw_method": "none", "animation_loop": "none", "color_scheme": "none", "responsiveness": "none", "interaction": "none" } },
    "svg_animations": { "enabled": false, "type": "none", "description": "Not used.", "params": { "animation_method": "none", "path_morphing": "none", "stroke_animation": "none", "filter_effects": "none" } },
    "composite_notes": "Use lightweight motion and product assets. Avoid decorative effects that compete with app icons, screenshots, source state, or admin forms."
  }
}
```

## Interaction Rules

- Every screen has one primary action. Secondary actions are visible but visually quieter.
- Add/edit/submit flows open from explicit buttons into a dialog, wizard, or full page. They should not occupy the default list view.
- App detail in the client is a full page or full-screen sheet behavior, not a narrow side drawer.
- Source filters and category filters are peers near the top of catalog views.
- Installed apps are local inventory rows/cards with fixed sizing, two-line name clamp, one-line app ID ellipsis, source attribution, and update badges.
- Updateable apps appear as a peer category to All. A bulk update button appears only when there are updateable apps.
- Source cards are compact. Edit/delete are available on hover for pointer devices and always reachable through an explicit menu or actions on touch devices.
- Server app cards use Download as the public action. Client app cards use Install, Reinstall, Update, or Rollback depending on local state and selected version.
- Comments support comment plus replies. Server maintainers/admins can delete comments. Client comment identity can use a display name without exposing the real user name.
- Non-LazyCat clients cannot comment.

## Component Boundary Consensus

Component splitting starts only after this consensus and the implementation plan are written. The split should follow product ownership:

- `shared`: API helpers, i18n helpers, formatting, hooks, layout primitives, reusable feedback components, app icon rendering, Astryx wrappers when needed.
- `storefront`: public server pages and presentation components.
- `admin`: server backstage pages, admin settings, taxonomy, collections, users, review queue, and maintainer workspace.
- `client`: source catalog, source management, installed inventory, updates, history, rollback, and client-only settings.
- `styles`: token layers and feature CSS modules or scoped CSS files imported from feature entry points.

Containers may fetch data and coordinate actions. Presentational components receive typed props and do not call global APIs directly. Forms keep their own draft state and emit validated submit payloads.

## Framework And Theming Rules

- Keep React, Vite, TypeScript, Astryx, lucide-react, and i18next.
- Prefer Astryx inputs, text areas, selectors, tabs, buttons, badges, and cards where they fit.
- Do not wrap native inputs inside custom input-looking containers. Remaining native file inputs must be visually styled or placed behind a styled Astryx-compatible file picker surface.
- Theme values come from CSS variables. Components should not introduce hardcoded color islands.
- Light and dark themes must cover sidebar, header, modals, hover states, form fields, cards, and empty states.
- All new visible strings go through `client/src/locales/zh.ts` and `client/src/locales/en.ts` or follow the planned locale split.
- Touch targets are at least 44px. Layout must work at 320, 375, 768, 1024, and 1440px.
- Animations must use transform/opacity/color/border-color, stay under 240ms for routine UI, and respect `prefers-reduced-motion`.

## Brainstorming Review

### Approaches Considered

| Approach | What It Means | Pros | Cons | Decision |
| --- | --- | --- | --- | --- |
| Feature-boundary refactor | Split by storefront, admin, client, and shared primitives | Matches product responsibilities and future maintenance | Requires deliberate data-flow seams | Use this |
| Atomic component library first | Create a large generic component library before pages | Clean in theory | High churn and delays visible UX fixes | Do not start here |
| CSS-only polish | Keep `App.tsx` mostly intact and restyle | Fast visual changes | Leaves page responsibility and huge-file issues unsolved | Use only for tactical fixes |

### Consensus Audit

- The public server, server backstage, and standalone client have distinct responsibilities.
- The component split is delayed until the page model and design DNA are explicit.
- The plan allows direct redesign because no historical frontend/data compatibility is required.
- The visual system is tokenized and avoids one-note purple/blue, beige, dark-slate, or decorative orb styles.
- The UI uses Astryx controls by default, matching the user's request to remove redundant native input styling.
- The plan covers theme switching, i18n, mobile widths, feedback states, and browser verification.
- No section relies on the server knowing client-installed applications.

### Default Decisions If The User Is Unavailable

- Proceed with the feature-boundary refactor.
- Keep the existing API surface unless the UI requires a small additional endpoint or type cleanup.
- Prefer fewer, higher-quality screens over preserving every current card or metric.
- Remove or hide cluttered status panels that do not answer the current screen's main question.

## Acceptance Criteria

- Public server UI opens to app discovery and source usage, not admin/profile/inventory panels.
- Logged-out server UI does not show "My Apps"; login is the entry to backstage.
- Server backend/admin app management can edit, classify, submit, and delete apps through scoped flows.
- Client can filter by source and source category, switch installed/synced views by source, see update badges, and bulk update when updates exist.
- Client source cards are compact and editable/deletable without oversized full-width rows.
- App detail in the client is full-page/full-screen, not a cramped side drawer.
- App icons render from parsed/uploaded metadata whenever available, with stable fallbacks.
- Theme switching changes header, sidebar, cards, forms, and modals consistently.
- Forms use Astryx components or styled file-picker surfaces, without nested native-input visual bugs.
- Browser checks pass at desktop and mobile widths with no overlapping text, horizontal scrolling, or inaccessible controls.
