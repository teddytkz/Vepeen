//go:build windows
// +build windows

package ui

import (
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

// centerOnWorkArea centers the window within the monitor's work area (the
// usable desktop rectangle excluding the taskbar), rather than against the
// full monitor rectangle that Fyne's CenterOnScreen uses. This avoids the
// window appearing offset downward on Windows.
//
// It uses SystemParametersInfoW with SPI_GETWORKAREA (0x30) to obtain the
// work-area bounds, then moves the native window with SetWindowPos. If the
// work-area query fails, or the native window handle is unavailable, it falls
// back to Fyne's CenterOnScreen.
//
// Note: at the time NewMainWindow runs the GLFW window has not been created
// yet, so RunNative reports a zero HWND. We therefore defer the move to a
// short-lived goroutine that retries until the native window exists.
func centerOnWorkArea(w fyne.Window) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	spi := user32.NewProc("SystemParametersInfoW")

	var rect windows.Rect
	ret, _, _ := spi.Call(0x30, 0, uintptr(unsafe.Pointer(&rect)), 0)
	if ret == 0 {
		// Work-area query failed; fall back to Fyne's default centering.
		w.CenterOnScreen()
		return
	}

	nw, ok := w.(driver.NativeWindow)
	if !ok {
		w.CenterOnScreen()
		return
	}

	workW := int(rect.Right) - int(rect.Left)
	workH := int(rect.Bottom) - int(rect.Top)

	setWindowPos := user32.NewProc("SetWindowPos")
	getWindowRect := user32.NewProc("GetWindowRect")

	// SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE: move only, keep size/z-order.
	const swpFlags = 0x0001 | 0x0004 | 0x0010

	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for {
			done := false
			nw.RunNative(func(ctx any) {
				wc, ok := ctx.(driver.WindowsWindowContext)
				if !ok || wc.HWND == 0 {
					return
				}

				var wr windows.Rect
				getWindowRect.Call(wc.HWND, uintptr(unsafe.Pointer(&wr)))
				winW := int(wr.Right) - int(wr.Left)
				winH := int(wr.Bottom) - int(wr.Top)
				if winW <= 0 || winH <= 0 {
					return
				}

				x := int(rect.Left) + (workW-winW)/2
				y := int(rect.Top) + (workH-winH)/2
				setWindowPos.Call(wc.HWND, 0, uintptr(x), uintptr(y), 0, 0, swpFlags)
				done = true
			})
			if done {
				return
			}
			if time.Now().After(deadline) {
				// Could not obtain a valid native window in time; give up
				// gracefully rather than block forever.
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
}
