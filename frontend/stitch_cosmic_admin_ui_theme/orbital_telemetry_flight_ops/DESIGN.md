---
name: Orbital Telemetry & Flight Ops
colors:
  surface: '#121318'
  surface-dim: '#121318'
  surface-bright: '#38393f'
  surface-container-lowest: '#0d0e13'
  surface-container-low: '#1a1b21'
  surface-container: '#1e1f25'
  surface-container-high: '#292a2f'
  surface-container-highest: '#34343a'
  on-surface: '#e3e1e9'
  on-surface-variant: '#bdc8d1'
  inverse-surface: '#e3e1e9'
  inverse-on-surface: '#2f3036'
  outline: '#87929a'
  outline-variant: '#3e484f'
  surface-tint: '#7bd0ff'
  primary: '#8ed5ff'
  on-primary: '#00354a'
  primary-container: '#38bdf8'
  on-primary-container: '#004965'
  inverse-primary: '#00668a'
  secondary: '#bdc2ff'
  on-secondary: '#131e8c'
  secondary-container: '#2f3aa3'
  on-secondary-container: '#a8afff'
  tertiary: '#4ee6aa'
  on-tertiary: '#003825'
  tertiary-container: '#22c990'
  on-tertiary-container: '#004e35'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#c4e7ff'
  primary-fixed-dim: '#7bd0ff'
  on-primary-fixed: '#001e2c'
  on-primary-fixed-variant: '#004c69'
  secondary-fixed: '#e0e0ff'
  secondary-fixed-dim: '#bdc2ff'
  on-secondary-fixed: '#000767'
  on-secondary-fixed-variant: '#2f3aa3'
  tertiary-fixed: '#68fcbf'
  tertiary-fixed-dim: '#45dfa4'
  on-tertiary-fixed: '#002114'
  on-tertiary-fixed-variant: '#005137'
  background: '#121318'
  on-background: '#e3e1e9'
  surface-variant: '#34343a'
typography:
  headline-xl:
    fontFamily: Space Grotesk
    fontSize: 40px
    fontWeight: '600'
    lineHeight: 48px
    letterSpacing: -0.02em
  headline-xl-mobile:
    fontFamily: Space Grotesk
    fontSize: 30px
    fontWeight: '600'
    lineHeight: 38px
    letterSpacing: -0.01em
  headline-lg:
    fontFamily: Space Grotesk
    fontSize: 32px
    fontWeight: '600'
    lineHeight: 40px
    letterSpacing: -0.015em
  headline-lg-mobile:
    fontFamily: Space Grotesk
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: -0.01em
  headline-md:
    fontFamily: Space Grotesk
    fontSize: 24px
    fontWeight: '500'
    lineHeight: 32px
  headline-sm:
    fontFamily: Space Grotesk
    fontSize: 18px
    fontWeight: '500'
    lineHeight: 26px
  body-lg:
    fontFamily: Geist
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
  body-md:
    fontFamily: Geist
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  body-sm:
    fontFamily: Geist
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 16px
  telemetry-lg:
    fontFamily: JetBrains Mono
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
    letterSpacing: 0.02em
  telemetry-md:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: '500'
    lineHeight: 20px
    letterSpacing: 0.03em
  telemetry-sm:
    fontFamily: JetBrains Mono
    fontSize: 11px
    fontWeight: '500'
    lineHeight: 14px
    letterSpacing: 0.05em
  label-md:
    fontFamily: Geist
    fontSize: 13px
    fontWeight: '500'
    lineHeight: 16px
    letterSpacing: 0.01em
  label-sm:
    fontFamily: Geist
    fontSize: 11px
    fontWeight: '600'
    lineHeight: 14px
    letterSpacing: 0.04em
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  gutter: 1rem
  gutter-desktop: 1.5rem
  margin: 1rem
  margin-desktop: 2rem
  space-xs: 0.25rem
  space-sm: 0.5rem
  space-md: 0.75rem
  space-lg: 1.25rem
  space-xl: 2rem
---

## Brand & Style

This design system establishes a high-density, mission-critical operational interface engineered for orbital telemetry, deep-space asset tracking, and internal technical administration. The aesthetic fuses aerospace cockpit precision with deep-space glassmorphism—evoking the hyper-controlled environment of a flight operations command deck.

The visual style leverages:
- **Atmospheric Dark Glassmorphism:** Translucent cosmic navy panels sitting atop deep-space void blacks, bound by precision hairline glow borders.
- **Instrument Precision:** High information density, fixed-width telemetry readouts, micro-status matrices, and structured categorical indicators.
- **Controlled Luminescence:** Restrained use of neon and spectral hues strictly mapped to telemetry deltas, tracking locks, and operational states, avoiding visual fatigue during prolonged low-light monitoring shifts.

## Colors

The palette is tuned specifically for prolonged deep-space monitoring, low-light operational environments, and immediate telemetry differentiation.

- **Primary (`#38bdf8` - Orbital Cyan):** Applied to active tracking vectors, key interactive affordances, active radio links, and focused operational states.
- **Secondary (`#818cf8` - Ion Purple):** Used for auxiliary computational readouts, secondary subsystems, orbital trajectories, and predictive telemetry paths.
- **Tertiary (`#34d399` - Laser Green):** Indicates nominal telemetry, positive telemetry syncs, confirmed locks, and operational readiness.
- **Neutral Core (`#0a0b10` - Void Black):** The primary canvas ground, representing deep space, minimizing OLED panel power draw and screen glare.
- **Surface Elevation Layers:**
  - Base Deck: `#0a0b10` (Void Black)
  - Subsystem Panels: `rgba(15, 23, 42, 0.75)` (Deep Cosmic Navy at 75% opacity with blur)
  - Interactive Surfaces & Hover States: `#1e293b` (Starry Slate)
  - Elevated Popovers & Control Overlays: `rgba(30, 41, 59, 0.85)`
- **Functional Telemetry Signals:**
  - Critical / Trajectory Breach: `#f87171` (Solar Flare Red)
  - Warning / Drift / Delta Deviation: `#fbbf24` (Starlight Amber)
  - Nominal / Lock Confirmed: `#34d399` (Laser Green)
  - Standby / Inactive: `#64748b` (Cosmic Dust Muted Slate)

## Typography

Typographic hierarchy separates contextual structure from live instrumentation data:

- **Display & Section Headers (`Space Grotesk`):** Technical yet geometric, giving headers an unmistakable aerospace engineering authority without compromising legibility.
- **Narrative & UI Text (`Geist`):** Neutral, crisp, and high-legibility at small scale. Used for admin controls, forms, descriptions, and workflow triggers.
- **Telemetry, Coordinates, & Timestamps (`JetBrains Mono`):** Fixed-width baseline ensures numerical stability. Data readouts (velocity, trajectory vector matrices, UTC time codes, transponder codes) will not cause layout shift during active stream updates.
- All uppercase telemetry labels and operational codes must feature expanded letter-spacing (`0.04em` to `0.05em`) for immediate perceptual scanning in critical incident monitoring.

## Layout & Spacing

Telemetry dashboards demand high-density layout architectures that balance maximal data visibility with zero visual clutter.

- **Layout Grid Model:** 
  - **Desktop (1440px+):** 12-column or 16-column continuous fluid layout utilizing `gutter-desktop` (1.5rem) and `margin-desktop` (2rem). Allows multi-channel parallel panels (e.g., Star Tracker viewport, Velocity Spectrograph, Thruster Diagnostics).
  - **Command Station / Ultra-Wide (1920px+):** Pinned operations navigation with multi-window tiling grids.
  - **Tactical Mobile / Tablet (360px - 1024px):** Single-column stacked telemetry ribbons; lower-priority streaming channels collapse into drawer modules using `gutter` (1rem) and `margin` (1rem).
- **Density Philosophy:** Compact standard. Inner component padding defaults to `space-sm` (0.5rem) or `space-md` (0.75rem) to maximize viewport real estate while maintaining 4px touch target alignment.

## Elevation & Depth

Visual depth is achieved through optical filtration and atmospheric stratification rather than traditional diffuse drop shadows.

1. **Flight Deck Canvas (Ground 0):** Pure Void Black (`#0a0b10`), non-reflective, flat base.
2. **Structural Console Panels (Level 1):** `rgba(15, 23, 42, 0.65)` layered with `backdrop-filter: blur(16px)` and a subtle single-pixel containment border: `1px solid rgba(56, 189, 248, 0.12)`.
3. **Focused / Armed Instrumentation (Level 2):** `rgba(30, 41, 59, 0.85)` with `backdrop-filter: blur(24px)`. Border illumination shifts to primary cyan (`rgba(56, 189, 248, 0.35)`) with a perimeter photon bleed: `box-shadow: 0 0 16px -2px rgba(56, 189, 248, 0.15)`.
4. **Flight Override Modals & Critical Alerts (Level 3):** Solidified cosmic slate backing `rgba(15, 23, 42, 0.95)`, framed with situational alarm glow: `0 0 24px -4px rgba(248, 113, 113, 0.3)` for critical vectors or `rgba(56, 189, 248, 0.25)` for system modals.

## Shapes

The design system employs a restrained, tactical geometric silhouette.

- **Roundedness Level 1 (Soft / Engineered Edge):** Standard components feature `0.25rem` (4px) radii, mimicking high-spec ruggedized telemetry displays and machined physical hardware.
- **Panels & Viewport Containers:** `rounded-lg` (0.5rem / 8px) to soften macro dashboard bounds without wasting peripheral scanning area.
- **Pill Exceptions:** Restricted strictly to operational status badges, radar lock indicators, and live telemetry transponder pings to reinforce their dynamic, capsule-like nature.

## Components

### Buttons & Flight Triggers
- **Primary Operational Button:** Solid orbital cyan (`#38bdf8`) background with deep void navy text (`#0a0b10`), bold weight. Subtle glow on hover (`box-shadow: 0 0 12px rgba(56, 189, 248, 0.4)`).
- **Secondary Subsystem Button:** Transparent background, `1px solid rgba(56, 189, 248, 0.3)`, text in orbital cyan. Background fills to `rgba(56, 189, 248, 0.08)` on pointer hover.
- **Destructive / Abort Action:** Hairline border `rgba(248, 113, 113, 0.4)`, solar red text, transitioning to solid `#f87171` fill with active red glow upon arming/holding.

### Telemetry Badges & Status Indicators
- Micro-pills rendered with monospace type (`telemetry-sm`).
- Built with high-contrast indicator dots possessing active pulse animations:
  - *Nominal Orbit:* Emerald indicator (`#34d399`) with matching ambient halo.
  - *Tracking Sync:* Pulsing cyan indicator (`#38bdf8`).
  - *Desync / Trajectory Error:* High-frequency flickering solar red indicator (`#f87171`).

### Telemetry Cards & Console Panels
- Frosted dark navy containers with hairline outlines (`rgba(56, 189, 248, 0.15)`).
- Card headers incorporate a tactical top-right coordinate or channel tag (e.g., `CH-04 // SAT-LAT`) rendered in muted `JetBrains Mono`.
- Sub-divided data grids inside cards utilize fine crosshair divider lines (`rgba(148, 163, 184, 0.1)`).

### Input Fields & Command Prompts
- Compact input boxes featuring deep navy backings (`#0f172a`), inset borders, and bright cyan focus outlines accompanied by a subtle focus glow.
- Monospace placeholder text in muted cosmic slate (`#64748b`).
- Integrated unit badges (e.g., `km/s`, `AU`, `GHz`) pinned to the trailing edge.

### Checkboxes, Switches & Radio Nodes
- **Switches:** Modeled after aerospace toggle banks; dark slate track with high-contrast cyan indicator knob that glows when engaged.
- **Checkboxes:** Square with chamfered inner check, framed in starry slate (`#1e293b`), shifting to primary orbital cyan fill with dark check icon when active.

### Specialized Telemetry Displays
- **Vector Sparklines:** Minimal hairline line charts with cyan-to-transparent vertical area fills.
- **Telemetry Readout Matrix:** Paired key-value rows; keys rendered in uppercase muted sans (`label-sm`), values rendered in high-legibility bold monospace (`telemetry-md`).