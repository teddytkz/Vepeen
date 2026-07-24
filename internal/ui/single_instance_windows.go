//go:build windows

package ui

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"golang.org/x/sys/windows"
)

const mutexName = "Global\\VepeenSingleInstance"

// showEventName is a named Win32 event used by a second instance to signal the
// first (owning) instance that it should show its window via Fyne. Showing via
// Fyne (rather than native ShowWindow) ensures Fyne repaints the canvas,
// avoiding the blank/white window bug when the window was hidden to tray.
const showEventName = "Global\\VepeenShowEvent"

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procAllowSetForegroundWindow = modUser32.NewProc("AllowSetForegroundWindow")
	procCreateEventW             = modKernel32.NewProc("CreateEventW")
	procOpenEventW               = modKernel32.NewProc("OpenEventW")
	procSetEvent                 = modKernel32.NewProc("SetEvent")
	procWaitForSingleObject      = modKernel32.NewProc("WaitForSingleObject")
)

const (
	asfwAny         = 0xFFFF     // AllowSetForegroundWindow: any process may set foreground
	waitInfinite    = 0xFFFFFFFF // WaitForSingleObject: wait forever
	eventModifyState = 0x0002    // EVENT_MODIFY_STATE — required to SetEvent
	synchronize      = 0x00100000 // SYNCHRONIZE — required to wait on the event
)

// AcquireSingleInstance tries to create a global named mutex.
// If another instance already holds it, it signals that instance to show its
// window and returns alreadyRunning=true. The caller should exit immediately
// when alreadyRunning is true.
// On the first (owning) instance it also creates the show event and returns a
// release func that closes both the mutex and the event handle.
func AcquireSingleInstance() (alreadyRunning bool, release func()) {
	name, _ := windows.UTF16PtrFromString(mutexName)
	handle, err := windows.CreateMutex(nil, false, name)

	// err is non-nil AND matches ERROR_ALREADY_EXISTS when another instance owns it.
	if err != nil && err == windows.ERROR_ALREADY_EXISTS {
		SignalExistingInstance()
		return true, func() { _ = windows.CloseHandle(handle) }
	}

	if handle == 0 {
		// Something went wrong creating the mutex — fail open (allow this instance).
		return false, func() {}
	}

	eventHandle, evErr := createShowEvent()
	if evErr != nil {
		// Mutex acquired but event creation failed — still release the mutex.
		return false, func() {
			_ = windows.CloseHandle(handle)
		}
	}

	return false, func() {
		_ = windows.CloseHandle(handle)
		_ = windows.CloseHandle(eventHandle)
	}
}

// createShowEvent creates (or opens) the named show event and returns its handle.
// If the event already exists (ERROR_ALREADY_EXISTS) we still get a valid handle
// to wait on, so that error is ignored.
func createShowEvent() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(showEventName)
	if err != nil {
		return 0, err
	}
	r, _, err := procCreateEventW.Call(
		0, // lpEventAttributes — nil
		0, // bManualReset — false (auto-reset)
		0, // bInitialState — false
		uintptr(unsafe.Pointer(name)),
	)
	h := windows.Handle(r)
	if h == 0 {
		return 0, err
	}
	// ERROR_ALREADY_EXISTS is fine — we still hold a valid handle to wait on.
	return h, nil
}

// SignalExistingInstance is best-effort: it opens the first instance's show event
// and sets it, asking the OS to allow the first instance to take foreground.
// Any failure is ignored — this is a UX convenience signal, never fatal.
func SignalExistingInstance() {
	name, err := windows.UTF16PtrFromString(showEventName)
	if err != nil {
		return
	}
	r, _, _ := procOpenEventW.Call(
		eventModifyState,
		0, // bInheritHandle — false
		uintptr(unsafe.Pointer(name)),
	)
	h := windows.Handle(r)
	if h == 0 {
		return
	}

	// Allow the first instance to take foreground. ASFW_ANY permits any process
	// to set the foreground window, which is sufficient and robust here.
	procAllowSetForegroundWindow.Call(asfwAny)

	procSetEvent.Call(uintptr(h))
	windows.CloseHandle(h)
}

// ListenForShowSignal opens/creates the show event and spawns a goroutine that
// waits on it. Each time the event is signaled (by a second instance launch),
// the window is shown via Fyne so the canvas repaints correctly. The goroutine
// loops forever so repeated launches keep working; it never blocks main.
func ListenForShowSignal(w fyne.Window) {
	name, err := windows.UTF16PtrFromString(showEventName)
	if err != nil {
		return
	}
	r, _, _ := procOpenEventW.Call(
		synchronize,
		0, // bInheritHandle — false
		uintptr(unsafe.Pointer(name)),
	)
	if r == 0 {
		// Event not yet created — create it so we can wait on it.
		r, _, _ = procCreateEventW.Call(
			0, 0, 0,
			uintptr(unsafe.Pointer(name)),
		)
	}
	h := windows.Handle(r)
	if h == 0 {
		return
	}

	go func() {
		for {
			procWaitForSingleObject.Call(uintptr(h), waitInfinite)
			fyne.Do(func() {
				w.Show()
				w.RequestFocus()
			})
		}
	}()
}
