# Vepeen brand assets

Vector artwork built from `docs/design/design_handoff_vepeen_logo/` (option 1c,
"Split Path"). `vepeen-mark.svg` is the master — everything else derives from it.

| File | Use |
|---|---|
| `vepeen-mark.svg` | Master. Badge + mark, 128×128, no padding. |
| `vepeen-mark-mono.svg` | Single-colour (`currentColor`), no badge. Direct branch at 50% opacity. |
| `vepeen-mark-16.svg` | 16px tier: stem + teal branch + one dot. Lower branch dropped. |
| `vepeen-wordmark.svg` | "Vepeen", SF Pro Bold, outlined. |
| `vepeen-lockup-dark.svg` / `-light.svg` | Horizontal mark + wordmark. |
| `Vepeen.icns` | macOS app icon (16 → 1024, @1x/@2x). |
| `menubar/vepeenTemplate.pdf` | Menu-bar template image (vector). `.png`/`@2x.png` alongside. |
| `favicon/` | 32px, 180px apple-touch, and SVG favicon. |
| `icons/` | Flat PNG renders, also the source for the Fyne bundle. |

## Two deviations from the handoff

**Branch angle.** The spec asks for ±38° branches *and* dots centred at
(100, 35) / (100, 93). Those are inconsistent — the stated dot centres imply
**34°**. The dot centres win here, because clear-space, the 128 grid and the
size tiers are all built around them; 4° is imperceptible, a misplaced dot is not.

**Light-badge fill.** The handoff offers `#0D6F66` or `#101A1C`. `#101A1C` is
used: the muted branch flattens to `#5A6B70`, which is **1.08:1** against
`#0D6F66` — effectively invisible. It is 3.18:1 on `#101A1C`.

## Regenerating

The mark is hand-authored SVG; edit `vepeen-mark.svg` and re-derive:

```sh
python3 <scratch>/gen.py      # mono, 16px, wordmark, lockups
bash   <scratch>/icons.sh     # PNG tiers + Vepeen.icns
go run <scratch>/gen/main.go assets/brand/icons/icon_1024.png > internal/ui/bundle_icon.go
```

The wordmark outlines come from CoreText using `.AppleSystemUIFontBold` — note
that `SFProDisplay-Bold` silently resolves to Helvetica on macOS.

`internal/ui/icon_test.go` asserts the bundled icon stays byte-identical to
`icons/icon_1024.png`, so a stale bundle fails the build rather than shipping.
