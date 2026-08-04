package main

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"

	"vepeen/internal/ui"
)

func main() {
	logPath := startupLogPath()
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		log.SetOutput(f)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
			panic(r)
		}
	}()

	// Single-instance guard: if another Vepeen is already running, bring its
	// window to the foreground and exit this new instance immediately.
	alreadyRunning, releaseMutex := ui.AcquireSingleInstance()
	if alreadyRunning {
		return
	}
	defer releaseMutex()

	// Unique app ID helps Fyne preferences; secrets use Windows Credential Manager separately.
	a := app.NewWithID("com.vepeen.app")
	a.SetIcon(ui.PenelopeIcon)
	a.Settings().SetTheme(ui.NewTheme())
	w, onQuit := ui.NewMainWindow(a)

	// System tray: icon in tray, X hides to tray, left-click shows window.
	ui.SetupTray(a, w, onQuit)

	// Listen for show signals from a second instance launch (tray-hidden window).
	ui.ListenForShowSignal(w)

	// ShowCentered centers the window (full monitor) and shows it before the
	// first frame, eliminating the startup blink. a.Run() only starts the loop.
	ui.ShowCentered(w)
	a.Run()
}

func startupLogPath() string {
	dir := os.Getenv("AppData")
	if dir == "" {
		dir = "."
	}
	_ = os.MkdirAll(filepath.Join(dir, "vepeen"), 0755)
	return filepath.Join(dir, "vepeen", "vepeen.log")
}
