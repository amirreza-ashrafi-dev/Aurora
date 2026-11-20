# Aurora Player — Premium Design System

This document records the visual specifications taken from the Aurora Figma file and the user's supplied reference images. Where a value has not yet been extracted from Figma, it is marked as an implementation estimate rather than a confirmed token.

## Brand

- Product: Aurora Cinema Player
- Display typeface target: Nimbus Sans
- Selected open alternative: TeX Gyre Heros (Nimbus Sans L-derived). Font files are not yet bundled; Arial remains the current system fallback until the font assets are added
- Primary logo: the complete 512 × 512 gold Aurora logo supplied separately by the user in chat. Do not substitute the older ZIP logo.
- Logo treatment: preserve the full composition, including the circular geometric mark and AURORA CINEMA PLAYER wordmark. The user explicitly requests the full logo for the taskbar icon too.

## Color palette

The following dark surface colors were sampled from the supplied screen references. Accent and text values are close visual matches; exact Figma variable values still need a complete variable/style export.

| Token | Value | Use |
|---|---|---|
| Header | #090807 | Custom title bar |
| Workspace | #0D0C0A | Main window background |
| Panel | #12110E | Queue and raised surfaces |
| Raised panel | #171511 | Secondary cards |
| Footer | #0B0A09 | Playback deck |
| Text | #EEEAE0 | Primary text |
| Muted text | #999181 | Secondary labels |
| Champagne gold | #DDBB53 | Primary actions and active state |
| Border | #332E22 | Warm low-contrast outlines |
| Gold-tinted surface | #2A2312 | Selected and highlighted controls |

## Shape and spacing

- Main stage and queue corner radius: 16 px in the Figma reference.
- Main workspace padding: 16 px.
- Stage-to-queue gap: 16 px.
- Main video / queue split at 1280 px: 912 px / 320 px.
- Queue width: 320 px.
- Main footer: approximately 170.5 px in the Figma reference.
- Custom header: 40 px.
- Use restrained 1 px warm borders and subtle shadows.
- Avoid purple/blue accents; use champagne gold consistently.

## Screen references

The Figma document has one page with four main frames:

| Frame | Node | Reference size | Purpose |
|---|---|---:|---|
| Fullscreen & Resume HUD | 1:2 | 1280 × 1024 | Fullscreen video, resume banner, floating HUD |
| Subtitle Studio Modal | 1:209 | 1280 × 1024 | Subtitle Studio over the player |
| Active Playback & Redesigned Queue | 1:500 | 1280 × 1024 | Main player with a populated queue |
| Luxe Gold Edition | 1:682 | 1280 × 1024 | Main player empty state |

### Main player frame

- Header: 1280 × 40.
- Main workspace: 1280 × 814.
- Video stage: 912 × 782.
- Queue panel: 320 × 782.
- Gap between stage and queue: 16.
- Stage and queue corner radius: 16.
- Footer: 1280 × 170.5.
- Footer horizontal padding: 24.

### Fullscreen frame

- Background: 1280 × 1024.
- Top overlay header: 1280 × 72.
- Continue Watching banner: 672 × 77 at (304, 96).
- Floating control deck: 896 × 194 at (192, 798).

### Subtitle Studio

Preserve the Figma section order and existing functionality:
1. Header: Load File, Disable/Enable, Close.
2. Font selection.
3. Font size and outline thickness.
4. Text color presets and custom color.
5. Outline color.
6. Vertical position.
7. Synchronization and reset.
8. Track management and language profile saving.
9. Explanatory note.

## Implementation rules

- Keep playback, MPV, queue, Smart Resume, subtitle persistence and Clean Architecture independent of the visual redesign.
- Use the same shared theme tokens in the main window, subtitle flyouts and fullscreen controller.
- Do not draw MyGo controls over the embedded MPV child HWND; use separate native overlay windows where controls must appear above video.
- Preserve the actual user-supplied logo; do not redraw or approximate it.
- Verify at the reference size 1280 × 1024 before testing other window sizes and Windows DPI scales.
- Treat responsive behavior as an adaptation of the reference, not a new visual design.

## Remaining asset work

- Add the exact logo image supplied separately in chat as a local embedded asset and create the required Windows icon resources from the full logo, preserving the full composition as requested.
- Bundle TeX Gyre Heros font files with their GUST Font License notice once the exact upstream distribution files have been added to the repository.
- Re-export exact Figma color, typography, gradient, shadow and component variables when available. Current values above distinguish sampled reference colors from confirmed measurements.
