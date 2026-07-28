# Handoff: Vepeen Logo — "Split Path" (option 1c)

## Overview
The chosen Vepeen logo is **1c "Split Path"**: a rounded-square badge containing one horizontal stem
entering from the left, branching into two diverging paths that end in two dots — the upper path and
dot are **teal** (encrypted, VPN tunnel), the lower path and dot are **muted white/gray** (direct
traffic). It is a literal picture of split tunneling, which is the product's defining feature.

The reference is `Vepeen Logo.dc.html` (option `1c`, plus its 40px lockup and a light-background
lockup for 1a — mirror the same lockup structure for 1c). It is a **design reference built in
HTML/CSS**, not production code. Rebuild it as **real vector artwork** (SVG master → all derived
assets). Do not ship the CSS-div construction.

## Fidelity
**High-fidelity.** Geometry, proportions, and colors below are the spec. Because the reference is
built from absolutely-positioned divs, treat the numbers as the intended geometry and redraw them
cleanly on a grid in vector form.

---

## Master geometry (draw on a 128 × 128 artboard)
The reference mark is 116×116; scale to a 128 grid with the badge inset by 0 (badge = full artboard).

- **Badge:** 128×128 rounded rectangle, corner radius **33** (≈26% — an Apple-style squircle;
  prefer a true squircle/superellipse path over a plain rounded rect for the macOS icon).
  Fill `#101A1C`. 1px inner stroke `rgba(255,255,255,.08)` on dark backgrounds; omit the stroke on
  light backgrounds.
- **Stem (entering path):** horizontal rounded bar, **width 33, height 11, radius 5.5**, left edge at
  **x = 24**, vertically centered (**y = 58.5**). Fill `#CFE0E2`.
- **Upper path (encrypted):** rounded bar **width 11, height 40, radius 5.5**, rotated **−38°**
  (rising to the right), its lower end joining the stem's right end at approximately **(57, 64)**.
  Fill `#2DD4BF`.
- **Lower path (direct):** the same bar mirrored — rotated **+38°** (falling to the right), lower end
  joining the same junction point. Fill `rgba(255,255,255,.35)`.
- **Upper dot:** circle **Ø13**, center at **(100, 35)**. Fill `#2DD4BF`, with an outer glow
  (Gaussian blur ~8, `#2DD4BF` at 60%) in the full-color version only.
- **Lower dot:** circle **Ø13**, center at **(100, 93)**. Fill `rgba(255,255,255,.35)`.
- **Optical rule:** the three strokes must meet in a single clean junction with matched round caps —
  no visible overlap seam, no gap. Build the stem + two branches as **one stroked path** with
  `stroke-linecap: round` and `stroke-linejoin: round` where possible, then split the color by
  drawing the teal branch as a second path on top.
- **Animation (product UI only, never in a static asset):** a ping ring on the upper dot — circle
  scaling 0.6 → 1.6 with opacity 0.9 → 0, 2.2s ease-out, infinite. Use only in the app's live UI
  (e.g. connected state), not in the icon.

**Recommended stroke-based construction** (cleaner than three shapes):
`M24,64 H57 M57,64 L88,40` (teal) and `M57,64 L88,88` (muted), stroke-width 11, round caps/joins,
then the two dots at the branch ends.

## Color
| Token | Value | Use |
|---|---|---|
| Badge fill | `#101A1C` | dark badge background |
| Badge stroke | `rgba(255,255,255,.08)` | 1px inner edge, dark contexts only |
| Encrypted teal | `#2DD4BF` | upper path + dot (the app's accent) |
| Teal deep | `#16B5A2` | gradient end / pressed states |
| Neutral stem | `#CFE0E2` | entering stem |
| Direct (muted) | `rgba(255,255,255,.35)` → flatten to `#5A6B70` on `#101A1C` | lower path + dot |
| Wordmark on dark | `#E8EEF0` | "Vepeen" |
| Wordmark on light | `#0B1416` | "Vepeen" |
| Light-badge fill | `#0D6F66` (or `#101A1C`) | badge on light backgrounds |

## Typography (wordmark)
- Face: system sans — **SF Pro Display / -apple-system** (substitute Inter Tight or General Sans if a
  licensed webfont is needed). **Weight 700.**
- **Letter-spacing −0.025em**, sentence case: **"Vepeen"** (never all-caps, never "VePeen").
- Lockup ratio: cap height of the wordmark ≈ **62%** of the badge height; badge-to-word gap =
  **0.33 × badge height** (reference: 40px badge, 13px gap, 25px type).
- Convert the final wordmark to outlines in the master asset.

## Required deliverables
1. **`vepeen-mark.svg`** — badge + mark, 128×128, no padding.
2. **`vepeen-mark-mono.svg`** — single-color (currentColor) version: no badge, strokes + dots only;
   the "direct" branch differentiated by **50% opacity**, not by hue. For menu-bar / template images.
3. **`vepeen-lockup-dark.svg`** and **`vepeen-lockup-light.svg`** — horizontal mark + wordmark.
4. **`vepeen-wordmark.svg`** — type only, outlined.
5. **macOS app icon set** — `.icns` built from 1024, 512, 256, 128, 64, 32, 16 px (@1x and @2x).
   Apple squircle silhouette; keep the artwork inside Apple's ~10% icon-grid margin (i.e. the mark
   sits on a 1024 canvas with the badge at ~824×824, centered, not full-bleed).
6. **`vepeen-menubar.pdf` / 16 & 32px PNG template images** — mono, macOS template naming
   (`…Template.pdf`) so the system tints it.
7. **Favicon** `32`, `180` (apple-touch), and an SVG favicon.

## Size simplification (mandatory)
The dots and the glow collapse below ~24px. Ship these tiers:
- **≥ 64px:** full mark, both dots, teal glow on the upper dot.
- **32px:** full mark, dots kept, **glow removed**, stroke width scaled proportionally (11/128).
- **24px:** drop the badge stroke; **increase stroke weight to ~13/128** and grow dots to Ø15/128 so
  the branch reads.
- **16px:** simplify to the **stem + upper (teal) branch + one dot only** — drop the lower branch.
  A 2-branch mark is mud at 16px. Verify by eye at 1× on a non-Retina display.

## Clear space & don'ts
- Clear space on all sides = **0.25 × badge height**.
- Minimum badge size: **16px** digital, **8mm** print.
- Don't: rotate the mark, re-color the branches to non-brand hues, place the teal branch below the
  muted one (direction carries meaning), add a drop shadow to the badge, outline the wordmark, or
  stretch the lockup non-uniformly.

## Where it is used in the product
- macOS app icon and Dock icon.
- Menu-bar item — use the **mono** version; tint teal when connected, default template gray when
  disconnected, amber `#F5B94A` while connecting.
- App title strip in the Vepeen window (see `Vepeen.dc.html` in the app handoff): 8px status dot +
  the wordmark today — swap the dot for the 16px mono mark.
- About panel, DMG background, notification icon.

## Files in this handoff
- `Vepeen Logo.dc.html` — logo exploration; option **1c** is the approved direction (also shows
  rejected 1a / 1b for context, and the small-size + light-background lockup pattern to mirror).
- `Vepeen.dc.html` — the app UI redesign the logo lives inside (palette source of truth).
