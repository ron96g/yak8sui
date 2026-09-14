# Apply a Deep Space Explorer theme to the existing UI

Use the visual language in `frontend/stitch_cosmic_admin_ui_theme` as inspiration for a new Material UI theme, retaining the resource workspace layout agreed in ADR 0021. The generated examples illustrate styling only: their dashboards, telemetry metrics, sample entities, branding, insignia, and navigation are not application requirements or assets to import.

## Palette

| Role | Token |
| --- | --- |
| Canvas / void navy | `#0A0E17` |
| Structural panel | `#111B2B` |
| Elevated surface | `#19263A` |
| Hover surface | `#203249` |
| Orbital cyan / primary action and focus | `#38BDF8` |
| Ion violet / secondary accent | `#A5B4FC` |
| Starlight / primary text | `#E6EDF7` |
| Muted text | `#A8B8CC` |
| Subtle divider | `#2B405A` |
| Control outline | `#58718F` |
| Success | `#34D399` |
| Warning / stale | `#FBBF24` |
| Error | `#F87171` |
| Unknown / neutral status | `#A8B8CC` |

Use dark canvas-colored text on bright filled actions/status chips and retain readable text labels alongside status colors. Info and live-connection accents may use cyan; health and connection state retain separate semantics. Check actual composed color contrast; subtle decorative dividers are not a substitute for visible interactive boundaries or keyboard focus.

## Typography and surfaces

- Space Grotesk for brand and headings, Geist for UI/body text, JetBrains Mono for identifiers, timestamps, technical metadata, and Monaco. Bundle font files locally with suitable fallbacks so the UI does not depend on third-party font requests at runtime.
- Use 14px body text and readable secondary labels, 4px control corners and 8px panel/modal corners. Keep the resource table compact and functional rather than turning it into dashboard cards.
- Establish hierarchy through layered navy surfaces and fine borders. Use restrained cyan emphasis for focus, selection, active tabs, and hover; violet is a secondary accent rather than a second status language.
- A subtle static atmospheric gradient is suitable for the login background or outer shell. Keep table/editor backgrounds opaque and legible. Avoid animated starfields, pulsing status effects, and high-glare decoration; respect reduced-motion preferences for any transitions.
- Apply tokens centrally through the MUI theme/component overrides, with targeted technical-text styling and a matching Monaco editor theme.

## Implementation and verification

Theme the existing login, tenant selector, navigation, filter panel, resource table, health/connection chips, conditions/YAML modal, menus, focus states, and empty/error/loading states. Preserve all data, labels, disabled future features, auth, SSE, and tenant/type/UID guards. Keep imported Stitch source files untouched and outside formatter/build inputs.

Verify formatting, typecheck/build, mandatory Go lint, existing browser journeys, and desktop/mobile theme rendering including modal, selected navigation, disabled inputs, keyboard focus, and status readability. This is a presentation change before ticket 06, not implementation of that ticket or the illustrative Stitch screens.
