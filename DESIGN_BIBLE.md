# Design Bible — Admin Portal

A visual and interaction spec for `web/index.html`, so every screen we add
later still feels like it came from the same hand. When in doubt, come back
to this file before adding a new color, shadow, or pattern. Periodically
re-check against well-executed civic/gov and ops-dashboard work on
dribbble.io (search: "government dashboard", "case management", "admin
portal ui") — borrow *proportions and hierarchy*, not literal styles; this
is a public-integrity tool, not a startup landing page.

## Design principles

1. **Trust over trend.** No gradients-for-the-sake-of-it, no glassmorphism,
   no playful illustrations. A case officer should feel this is as serious
   as a bank's back office, because it effectively is one.
2. **Density with air.** Government users work through long tables. Rows
   are compact, but grouped with generous section spacing so the eye has
   somewhere to rest.
3. **Status is a color you can read from across the room.** Every report
   and official has a status pill. The same status always uses the same
   color everywhere in the app — table, detail view, dashboard chart.
4. **One primary action per screen.** Never two competing buttons of equal
   visual weight.
5. **Everything works with just a keyboard and a bad mouse.** Focus states
   are always visible. Nothing depends on hover alone.

## Color tokens

```css
:root {
  /* Neutrals — the actual UI surface */
  --c-bg:        #F5F7FA;
  --c-surface:   #FFFFFF;
  --c-surface-2: #F0F2F6;
  --c-border:    #E2E6ED;
  --c-text:      #101728;
  --c-text-2:    #4B5568;
  --c-text-3:    #8892A4;

  /* Brand — civic blue, restrained */
  --c-primary:      #14508C;
  --c-primary-600:  #0F3E6D;
  --c-primary-50:   #E9F1FA;

  /* Status semantics — used ONLY for status, never decoration */
  --c-pending:    #B7791F;  --c-pending-bg:    #FDF3E1;
  --c-review:     #2B6CB0;  --c-review-bg:     #E8F1FC;
  --c-resolved:   #1A7F52;  --c-resolved-bg:   #E5F6EE;
  --c-dismissed:  #6B7280;  --c-dismissed-bg:  #EEF0F3;
  --c-danger:     #B42318;  --c-danger-bg:     #FBEAE9;

  /* Verification semantics (officials) */
  --c-verified:      #1A7F52;
  --c-unverified:    #B7791F;
  --c-investigation: #B42318;
}

@media (prefers-color-scheme: dark) {
  :root {
    --c-bg:        #0B0F17;
    --c-surface:   #121826;
    --c-surface-2: #1A2131;
    --c-border:    #262E42;
    --c-text:      #EDEFF4;
    --c-text-2:    #AEB6C6;
    --c-text-3:    #7C8497;
    --c-primary-50: #12233A;
  }
}
```

## Type

- Font: system UI stack — `-apple-system, "Segoe UI", Roboto, "Inter",
  sans-serif`. No webfont dependency (must render instantly on a weak LAN in
  a county office).
- Scale: 12 / 13 / 14 (body) / 16 / 20 / 24 / 32px. Never invent a size
  outside this scale.
- Weight: 400 body, 600 for labels/headers, 700 only for the single page
  title. Never bold a whole paragraph for emphasis — use `--c-text` vs
  `--c-text-2` instead.

## Spacing & shape

- 4px base unit. Spacing values: 4, 8, 12, 16, 24, 32, 48.
- Corner radius: 8px for cards/inputs, 6px for buttons, 999px (pill) for
  status badges only.
- Shadows are whisper-quiet: `0 1px 2px rgba(16,23,40,0.06), 0 1px 3px
  rgba(16,23,40,0.08)`. Never a shadow heavier than that — this is not a
  marketing site.
- Border over shadow where possible: `1px solid var(--c-border)` is the
  default card treatment; shadow is reserved for floating elements
  (dropdowns, modals, toasts).

## Components

- **Status pill**: `padding: 2px 10px; border-radius: 999px; font-size:
  12px; font-weight: 600;` background/text pair from the status tokens
  above, always paired (never colored text on a plain background).
- **Card**: white surface, 1px border, 8px radius, 16–24px padding.
- **Primary button**: `--c-primary` fill, white text, 6px radius, 10px/16px
  padding, `:hover` darkens to `--c-primary-600`, `:focus-visible` gets a
  2px `--c-primary-50`-derived ring.
- **Secondary button**: transparent fill, `--c-border` outline, `--c-text`
  label.
- **Table**: sticky header, row height 44px, zebra via `--c-surface-2` on
  even rows is *not* used — instead a 1px bottom border per row keeps it
  calm; row hover gets `--c-surface-2`.
- **Sidebar nav**: fixed 220px, `--c-surface`, icons + label, active item
  gets `--c-primary-50` background + `--c-primary` text + a 3px left
  accent bar.
- **Empty states**: never a blank table. One line of `--c-text-3` copy
  explaining what will appear here once it exists.
- **Toasts**: bottom-right, auto-dismiss 4s, one at a time (queue, don't
  stack).

## Motion

- 150ms ease-out for hover/focus transitions. 200ms for panel open/close.
  Nothing loops, nothing auto-plays, nothing bounces. A case officer
  reviewing evidence of corruption should never see something "delightful."

## Layout skeleton

```
┌──────────┬─────────────────────────────────────────┐
│          │  Topbar: page title · search · user menu │
│ Sidebar  ├─────────────────────────────────────────┤
│ 220px    │                                           │
│          │  Content: 24px padding, max-width 1280px, │
│  Nav     │  centered on wide screens                 │
│  items   │                                           │
│          │                                           │
└──────────┴─────────────────────────────────────────┘
```
Collapses to a top nav bar + hamburger under 768px — this must work on the
cheap laptop a sub-county records office actually owns.

## Content tone

- Plain English/Swahili, no jargon. "Mark as resolved," not "Update
  disposition."
- Numbers are never bare — always paired with a label: "12 pending", not
  "12".
- Dates are absolute (`24 Sep 2026, 14:03`) in tables, relative ("2 hours
  ago") only in dashboard summary cards where precision matters less.
