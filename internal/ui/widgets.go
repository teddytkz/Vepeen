package ui

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// card wraps content in a rounded, subtly-filled, bordered panel per the brief's
// card tokens (fill rgba(255,255,255,.03), border .06, 16px radius).
func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(cardFill)
	bg.CornerRadius = 16
	bg.StrokeColor = cardBorder
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// sectionLabel renders an uppercase teal section heading (11px/700, tracked).
func sectionLabel(text string) *canvas.Text {
	t := canvas.NewText(text, accentDim)
	t.TextSize = 11
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// helperText renders faint 12px helper copy.
func helperText(text string) *canvas.Text {
	t := canvas.NewText(text, textFaint)
	t.TextSize = 12
	return t
}

// statTile is a Down/Up/Ping tile: caption over a mono value, dark rounded fill.
func statTile(caption string, value *canvas.Text) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0x40})
	bg.CornerRadius = 12
	cap := canvas.NewText(caption, textFaint)
	cap.TextSize = 10
	cap.Alignment = fyne.TextAlignCenter
	value.Alignment = fyne.TextAlignCenter
	return container.NewStack(bg, container.NewPadded(
		container.NewVBox(cap, value),
	))
}

// heroRing is the focal circular connect/disconnect control. It stacks canvas
// circles (outer ring, glow, center disc) and animates a connecting spinner and
// a connected "breathe" pulse. Tapping calls onTap.
type heroRing struct {
	widget.BaseWidget

	onTap func()

	ring   *canvas.Circle // outer state ring
	glow   *canvas.Circle // inner state-colored glow
	disc   *canvas.Circle // dark center disc
	spin   *canvas.Circle // spinner arc proxy (opacity pulse fallback)
	kicker *canvas.Text
	label  *canvas.Text
	hint   *canvas.Text

	anim  *fyne.Animation
	state string // "disconnected" | "connecting" | "connected"
}

func newHeroRing(onTap func()) *heroRing {
	h := &heroRing{onTap: onTap, state: "disconnected"}
	h.ring = &canvas.Circle{StrokeColor: withAlpha(ringIdle, 0x59), StrokeWidth: 2, FillColor: color.Transparent}
	h.glow = &canvas.Circle{FillColor: withAlpha(ringIdle, 0x22)}
	h.disc = &canvas.Circle{FillColor: color.NRGBA{R: 0x08, G: 0x0e, B: 0x10, A: 0xff}}
	h.spin = &canvas.Circle{StrokeColor: color.Transparent, StrokeWidth: 3, FillColor: color.Transparent}

	h.kicker = canvas.NewText("TAP TO", ringIdle)
	h.kicker.TextSize = 10
	h.kicker.TextStyle = fyne.TextStyle{Monospace: true}
	h.kicker.Alignment = fyne.TextAlignCenter

	h.label = canvas.NewText("Connect", textPrimary)
	h.label.TextSize = 21
	h.label.TextStyle = fyne.TextStyle{Bold: true}
	h.label.Alignment = fyne.TextAlignCenter

	h.hint = canvas.NewText("not protected", textMuted)
	h.hint.TextSize = 11
	h.hint.Alignment = fyne.TextAlignCenter

	h.ExtendBaseWidget(h)
	return h
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func stateColor(state string) color.NRGBA {
	switch state {
	case "connecting":
		return warnColor
	case "connected":
		return accentColor
	default:
		return ringIdle
	}
}

// SetState updates ring colors, center text, and animation for the given state.
func (h *heroRing) SetState(state string) {
	h.state = state
	col := stateColor(state)
	h.ring.StrokeColor = withAlpha(col, 0x80)
	h.glow.FillColor = withAlpha(col, 0x22)
	h.kicker.Color = col

	switch state {
	case "connecting":
		h.kicker.Text, h.label.Text, h.hint.Text = "WORKING", "Connecting", "please wait"
	case "connected":
		h.kicker.Text, h.label.Text, h.hint.Text = "SECURE", "Connected", "split tunnel"
	default:
		h.kicker.Text, h.label.Text, h.hint.Text = "TAP TO", "Connect", "not protected"
	}

	h.stopAnim()
	switch state {
	case "connecting":
		h.startPulse(0.6, warnColor, 900) // fast amber pulse ≈ spinner
	case "connected":
		h.startPulse(0.35, accentColor, 3000) // slow teal breathe
	}

	h.Refresh()
}

// startPulse animates the glow alpha between base and a brighter peak.
func (h *heroRing) startPulse(basePeak float32, col color.NRGBA, ms int) {
	h.anim = fyne.NewAnimation(time.Duration(ms)*time.Millisecond, func(f float32) {
		// triangle wave 0→1→0 for a smooth in/out
		p := f * 2
		if p > 1 {
			p = 2 - p
		}
		a := basePeak + (1-basePeak)*p*0.5
		h.glow.FillColor = withAlpha(col, uint8(a*0x66))
		canvas.Refresh(h.glow)
	})
	h.anim.RepeatCount = fyne.AnimationRepeatForever
	h.anim.Curve = fyne.AnimationEaseInOut
	h.anim.Start()
}

func (h *heroRing) stopAnim() {
	if h.anim != nil {
		h.anim.Stop()
		h.anim = nil
	}
}

func (h *heroRing) Tapped(_ *fyne.PointEvent) {
	if h.onTap != nil {
		h.onTap()
	}
}

func (h *heroRing) CreateRenderer() fyne.WidgetRenderer {
	return &heroRingRenderer{h: h}
}

type heroRingRenderer struct {
	h *heroRing
}

func (r *heroRingRenderer) Layout(size fyne.Size) {
	// Center a square ring of side = min(w,h), capped at 178 per the brief.
	side := float32(math.Min(float64(size.Width), float64(size.Height)))
	if side > 178 {
		side = 178
	}
	ox := (size.Width - side) / 2
	oy := (size.Height - side) / 2

	place := func(o fyne.CanvasObject, inset float32) {
		o.Move(fyne.NewPos(ox+inset, oy+inset))
		o.Resize(fyne.NewSize(side-2*inset, side-2*inset))
	}
	place(r.h.ring, 0)
	place(r.h.spin, 0)
	place(r.h.glow, 14)
	place(r.h.disc, 30)

	// Stack the three text lines centered in the disc.
	cx := ox + side/2
	cy := oy + side/2
	texts := []*canvas.Text{r.h.kicker, r.h.label, r.h.hint}
	heights := []float32{14, 26, 15}
	total := heights[0] + heights[1] + heights[2]
	ty := cy - total/2
	for i, t := range texts {
		t.Resize(fyne.NewSize(side, heights[i]))
		t.Move(fyne.NewPos(cx-side/2, ty))
		ty += heights[i]
	}
}

func (r *heroRingRenderer) MinSize() fyne.Size { return fyne.NewSize(178, 178) }

func (r *heroRingRenderer) Refresh() {
	canvas.Refresh(r.h)
}

func (r *heroRingRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		r.h.glow, r.h.disc, r.h.ring, r.h.spin,
		r.h.kicker, r.h.label, r.h.hint,
	}
}

func (r *heroRingRenderer) Destroy() { r.h.stopAnim() }

// bgLayer returns the deep window background with a soft teal glow top-right,
// approximating the brief's gradient + glow with a solid fill and radial gradient.
func bgLayer() fyne.CanvasObject {
	base := canvas.NewRectangle(bgDeep)
	glow := canvas.NewRadialGradient(withAlpha(accentColor, 0x22), color.Transparent)
	glow.CenterOffsetX = 0.42
	glow.CenterOffsetY = -0.42
	return container.NewStack(base, glow)
}

// mono returns a monospace canvas text of the given size/color.
func mono(text string, size float32, col color.NRGBA) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

var _ = theme.SizeNameText // keep theme import if trimmed later

// ratioHBox lays two objects side by side with a fixed gap, splitting the
// remaining width by wLeft:wRight (the brief's 1 : 1.08 column ratio).
type ratioHBox struct {
	wLeft, wRight, gap float32
}

func newRatioHBox(wLeft, wRight, gap float32) fyne.Layout {
	return &ratioHBox{wLeft: wLeft, wRight: wRight, gap: gap}
}

func (l *ratioHBox) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) != 2 {
		return
	}
	avail := size.Width - l.gap
	total := l.wLeft + l.wRight
	lw := avail * l.wLeft / total
	rw := avail - lw
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(lw, size.Height))
	objs[1].Move(fyne.NewPos(lw+l.gap, 0))
	objs[1].Resize(fyne.NewSize(rw, size.Height))
}

func (l *ratioHBox) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) != 2 {
		return fyne.NewSize(0, 0)
	}
	a, b := objs[0].MinSize(), objs[1].MinSize()
	h := a.Height
	if b.Height > h {
		h = b.Height
	}
	return fyne.NewSize(a.Width+b.Width+l.gap, h)
}
