package ui

import (
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// card wraps content in a rounded, subtly-filled, bordered panel with 18px inner
// padding (design card tokens: fill rgba(255,255,255,.025), border .06, r16).
func card(content fyne.CanvasObject) fyne.CanvasObject {
	return cardPad(content, 18, cardFill, 16)
}

// cardPad is card() with explicit inner padding, fill, and corner radius.
func cardPad(content fyne.CanvasObject, pad float32, fill color.NRGBA, radius float32) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = radius
	bg.StrokeColor = cardBorder
	bg.StrokeWidth = 1
	padded := container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), content)
	return container.NewStack(bg, padded)
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
	// Two visible ring layers give the control depth (fix #3): a stroked outer
	// ring and an inner state-colored glow, over a dark center disc.
	h.ring = &canvas.Circle{StrokeColor: withAlpha(ringIdle, 0x66), StrokeWidth: 2, FillColor: color.Transparent}
	h.glow = &canvas.Circle{FillColor: withAlpha(ringIdle, 0x33)}
	h.disc = &canvas.Circle{FillColor: color.NRGBA{R: 0x0b, G: 0x13, B: 0x15, A: 0xff}}
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

// bgLayer returns the deep window background with a CONTAINED teal glow pinned to
// the top-right corner, so the left column stays near-neutral dark (fix #2: a
// full-size gradient tinted the whole window green).
func bgLayer() fyne.CanvasObject {
	base := canvas.NewRectangle(bgMid)
	glow := canvas.NewRadialGradient(withAlpha(accentColor, 0x26), color.Transparent)
	glow.Resize(fyne.NewSize(700, 340))
	// Pin to top-right via a border layout with a fixed-size glow in the top slot,
	// right-aligned. GridWrap fixes the glow's size; HBox spacer pushes it right.
	pinned := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), container.NewGridWrap(fyne.NewSize(700, 340), glow)),
		layout.NewSpacer(),
	)
	return container.NewStack(base, pinned)
}

// mono returns a monospace canvas text of the given size/color.
func mono(text string, size float32, col color.NRGBA) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

var _ = theme.SizeNameText // keep theme import if trimmed later

// tealCheck is the design's custom checkbox: a 20px rounded square, teal fill +
// dark check when on, faint border when off, with a label. Tappable.
type tealCheck struct {
	widget.BaseWidget
	Checked   bool
	OnChanged func(bool)
	label     string
}

func newTealCheck(label string, changed func(bool)) *tealCheck {
	c := &tealCheck{label: label, OnChanged: changed}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tealCheck) SetChecked(v bool) {
	c.Checked = v
	c.Refresh()
}

func (c *tealCheck) Tapped(_ *fyne.PointEvent) {
	c.Checked = !c.Checked
	c.Refresh()
	if c.OnChanged != nil {
		c.OnChanged(c.Checked)
	}
}

func (c *tealCheck) CreateRenderer() fyne.WidgetRenderer {
	box := canvas.NewRectangle(color.Transparent)
	box.CornerRadius = 6
	box.StrokeWidth = 1.5
	mark := canvas.NewText("✓", darkOnAccent)
	mark.TextStyle = fyne.TextStyle{Bold: true}
	mark.TextSize = 12
	mark.Alignment = fyne.TextAlignCenter
	lbl := canvas.NewText(c.label, textSecondary)
	lbl.TextSize = 13.5
	r := &tealCheckRenderer{c: c, box: box, mark: mark, lbl: lbl}
	r.Refresh()
	return r
}

type tealCheckRenderer struct {
	c    *tealCheck
	box  *canvas.Rectangle
	mark *canvas.Text
	lbl  *canvas.Text
}

func (r *tealCheckRenderer) Refresh() {
	if r.c.Checked {
		r.box.FillColor = accentColor
		r.box.StrokeColor = accentColor
		r.mark.Text = "✓"
	} else {
		r.box.FillColor = color.Transparent
		r.box.StrokeColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x38} // rgba(255,255,255,.22)
		r.mark.Text = ""
	}
	canvas.Refresh(r.box)
	canvas.Refresh(r.mark)
	canvas.Refresh(r.lbl)
}

func (r *tealCheckRenderer) Layout(size fyne.Size) {
	const bs = 20
	r.box.Resize(fyne.NewSize(bs, bs))
	r.box.Move(fyne.NewPos(0, (size.Height-bs)/2))
	r.mark.Resize(fyne.NewSize(bs, bs))
	r.mark.Move(fyne.NewPos(0, (size.Height-bs)/2))
	r.lbl.Move(fyne.NewPos(bs+10, (size.Height-r.lbl.MinSize().Height)/2))
	r.lbl.Resize(fyne.NewSize(size.Width-bs-10, r.lbl.MinSize().Height))
}

func (r *tealCheckRenderer) MinSize() fyne.Size {
	lw := r.lbl.MinSize().Width
	return fyne.NewSize(20+10+lw, 24)
}

func (r *tealCheckRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.box, r.mark, r.lbl}
}

func (r *tealCheckRenderer) Destroy() {}

// passwordWithToggle returns a password entry overlaid with a trailing mono
// "show"/"hide" button that flips masking. The entry is returned so the caller
// can read/set its text.
func passwordWithToggle() (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewPasswordEntry()
	e.SetPlaceHolder("Password")
	toggle := widget.NewButton("show", nil)
	toggle.Importance = widget.LowImportance
	toggle.OnTapped = func() {
		e.Password = !e.Password
		if e.Password {
			toggle.SetText("show")
		} else {
			toggle.SetText("hide")
		}
		e.Refresh()
	}
	// Border layout: entry fills, toggle pinned right.
	return e, container.NewBorder(nil, nil, nil, toggle, e)
}

// primaryButton styles a teal-filled CTA with dark text (design footer CTA).
func primaryButton(label string, tapped func()) *widget.Button {
	b := widget.NewButton(label, tapped)
	b.Importance = widget.HighImportance
	return b
}

// logKind selects a row color for the activity log.
type logKind int

const (
	logInfo logKind = iota
	logOK
	logWarn
	logMuted
)

func (k logKind) color() color.NRGBA {
	switch k {
	case logOK:
		return accentColor
	case logWarn:
		return warnColor
	case logMuted:
		return monoFaint
	default:
		return textSecondary // info #b9c6c9 (fix #4: not the faint gray)
	}
}

// logView is a scrollable, per-row-colored activity log (fix #4). Each row is a
// dim mono timestamp + a colored mono message. Capped; auto-scrolls to bottom.
type logView struct {
	widget.BaseWidget
	rows   *fyne.Container // VBox of row containers
	scroll *container.Scroll
	cap    int
}

func newLogView(capLines int) *logView {
	l := &logView{rows: container.NewVBox(), cap: capLines}
	// Scroll in BOTH directions: a long single line scrolls horizontally inside
	// the box instead of forcing the whole window wider (the connected-state
	// blowout bug — a raw diagnostics line was unbreakable and unbounded).
	l.scroll = container.NewScroll(l.rows)
	l.ExtendBaseWidget(l)
	return l
}

// Append adds a "HH:MM:SS  message" row in the kind's color and scrolls to bottom.
func (l *logView) Append(ts, msg string, kind logKind) {
	t := mono(ts, 12, monoFaint) // timestamp #4a5a5e
	m := mono(msg, 12, kind.color())
	l.rows.Add(container.NewHBox(t, m))
	if len(l.rows.Objects) > l.cap {
		l.rows.Remove(l.rows.Objects[0])
	}
	l.rows.Refresh()
	l.scroll.ScrollToBottom()
}

func (l *logView) Clear() {
	l.rows.RemoveAll()
	l.rows.Refresh()
}

func (l *logView) CreateRenderer() fyne.WidgetRenderer {
	return &logViewRenderer{l: l}
}

// logViewRenderer clamps the log's MinSize to a modest fixed box so its scrolling
// content (which can contain very long lines) never dictates the window width.
type logViewRenderer struct{ l *logView }

func (r *logViewRenderer) Layout(size fyne.Size)        { r.l.scroll.Resize(size) }
func (r *logViewRenderer) MinSize() fyne.Size           { return fyne.NewSize(240, 120) }
func (r *logViewRenderer) Refresh()                     { r.l.scroll.Refresh() }
func (r *logViewRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.l.scroll} }
func (r *logViewRenderer) Destroy()                     {}

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
