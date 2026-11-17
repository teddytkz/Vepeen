//go:build windows

package ui

import (
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

var (
	procGetSystemMetrics      = modUser32.NewProc("GetSystemMetrics")
	procSystemParametersInfoW = modUser32.NewProc("SystemParametersInfoW")
	procIsWindowVisible       = modUser32.NewProc("IsWindowVisible")
)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
	smCMonitors       = 80
	spiGetWorkArea    = 0x0030

	// Target window size (logical) from NewMainWindow, and the room to leave
	// for the title bar / borders (physical px) when shrinking to the work area.
	defaultW, defaultH = 1040, 780
	chromeW, chromeH   = 16, 48
)

type winRect struct{ left, top, right, bottom int32 }

// displaySig identifies the current desktop geometry (resolution, origin, monitor count).
func displaySig() [5]int {
	var s [5]int
	for i, m := range [...]uintptr{smXVirtualScreen, smYVirtualScreen, smCXVirtualScreen, smCYVirtualScreen, smCMonitors} {
		v, _, _ := procGetSystemMetrics.Call(m)
		s[i] = int(int32(v))
	}
	return s
}

// WatchDisplayChanges re-fits the window after the desktop geometry changes
// (RDP connect/disconnect, resolution or monitor change). Fyne only re-detects
// its scale when the window is moved and never re-fits the size, so after such a
// change the window keeps a stale scale/size and can end up bigger than the
// screen. We poll (no message loop is exposed by Fyne) and apply the fix once the
// window is actually visible, so a window hidden in the tray is fixed on Show.
func WatchDisplayChanges(a fyne.App, w fyne.Window) {
	applied := displaySig()
	go func() {
		for range time.Tick(time.Second) {
			fyne.Do(func() {
				sig := displaySig()
				if sig == applied || !windowVisible(w) {
					return
				}
				applied = sig
				refit(a, w)
			})
		}
	}()
}

func windowVisible(w fyne.Window) bool {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return false
	}
	visible := false
	nw.RunNative(func(c any) {
		if ctx, ok := c.(driver.WindowsWindowContext); ok && ctx.HWND != 0 {
			r, _, _ := procIsWindowVisible.Call(ctx.HWND)
			visible = r != 0
		}
	})
	return visible
}

func refit(a fyne.App, w fyne.Window) {
	// Re-applying the theme makes Fyne recompute the DPI scale for the new monitor mode.
	a.Settings().SetTheme(a.Settings().Theme())

	// shortcut: primary-monitor work area only, upgrade if multi-monitor sizing matters.
	var wa winRect
	size := fyne.NewSize(defaultW, defaultH)
	if r, _, _ := procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&wa)), 0); r != 0 {
		s := w.Canvas().Scale()
		size.Width = min(size.Width, float32(wa.right-wa.left-chromeW)/s)
		size.Height = min(size.Height, float32(wa.bottom-wa.top-chromeH)/s)
	}
	w.Resize(size)
	w.CenterOnScreen()
}
