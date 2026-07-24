//go:build windows

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// COM / shell constants for creating a Windows desktop shortcut (.lnk).
const (
	csidlDesktopDirectory = 0x10 // CSIDL_DESKTOPDIRECTORY
	clsctxAll             = 0x17 // CLSCTX_ALL

	coinitApartmentThreaded = 0x2 // COINIT_APARTMENTTHREADED
	coinitDisableOle1Dde    = 0x4 // COINIT_DISABLE_OLE1DDE

	maxPath = 260 // MAX_PATH
)

// GUIDs for the ShellLink coclass and the IShellLinkW / IPersistFile interfaces.
var (
	clsidShellLink = windows.GUID{
		Data1: 0x00021401,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	iidIShellLinkW = windows.GUID{
		Data1: 0x000214F9,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	iidIPersistFile = windows.GUID{
		Data1: 0x0000010B,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
)

var (
	modOle32   = windows.NewLazySystemDLL("ole32.dll")
	modShell32 = windows.NewLazySystemDLL("shell32.dll")

	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procSHGetFolderPathW = modShell32.NewProc("SHGetFolderPathW")
)

// comPtr wraps a raw COM interface pointer (a pointer to the vtable). The
// pointer is kept as unsafe.Pointer so it is visible to the GC and never
// converted through uintptr (which would break vet's unsafe rules).
type comPtr struct {
	p unsafe.Pointer
}

// call invokes a vtable method by index, passing the interface pointer as the
// implicit "this" argument followed by args. It returns the raw HRESULT.
func (c comPtr) call(methodIndex uint32, args ...uintptr) uintptr {
	vtable := *(*unsafe.Pointer)(c.p)
	methodPtr := (*unsafe.Pointer)(unsafe.Add(unsafe.Pointer(vtable),
		uintptr(methodIndex)*unsafe.Sizeof(uintptr(0))))
	method := *methodPtr
	ret, _, _ := syscall.SyscallN(uintptr(method), append([]uintptr{uintptr(c.p)}, args...)...)
	return ret
}

// release calls IUnknown.Release (vtable index 2).
func (c comPtr) release() {
	c.call(2)
}

// CreateDesktopShortcut creates a Windows desktop shortcut (.lnk) pointing at
// the running executable. If the shortcut already exists it returns nil
// without overwriting. Non-Windows builds return nil (no-op).
func CreateDesktopShortcut() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	desktop, err := getDesktopPath()
	if err != nil {
		return fmt.Errorf("resolve desktop path: %w", err)
	}

	lnkPath := filepath.Join(desktop, "Vepeen.lnk")
	if _, statErr := os.Stat(lnkPath); statErr == nil {
		// Already exists — skip silently (caller logs "already exists").
		return nil
	}

	if err := createShellLink(lnkPath, exePath); err != nil {
		return fmt.Errorf("create shell link: %w", err)
	}
	return nil
}

// getDesktopPath returns the current user's Desktop directory via
// SHGetFolderPathW with CSIDL_DESKTOPDIRECTORY.
func getDesktopPath() (string, error) {
	buf := make([]uint16, maxPath)
	r, _, _ := procSHGetFolderPathW.Call(
		0, // hwnd (NULL)
		uintptr(csidlDesktopDirectory),
		0, // hToken (NULL)
		0, // dwFlags
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if r != 0 {
		return "", fmt.Errorf("SHGetFolderPathW failed (hresult 0x%x)", uint32(r))
	}
	return windows.UTF16ToString(buf), nil
}

// createShellLink builds the .lnk file using IShellLinkW + IPersistFile COM.
func createShellLink(lnkPath, exePath string) error {
	r, _, _ := procCoInitializeEx.Call(0, uintptr(coinitApartmentThreaded|coinitDisableOle1Dde))
	if r == 0 {
		defer procCoUninitialize.Call()
	} else if r != 1 { // S_FALSE = already initialized, do not uninitialize
		return fmt.Errorf("CoInitializeEx failed (hresult 0x%x)", uint32(r))
	}

	var shellLink unsafe.Pointer
	r, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0, // pUnkOuter (NULL)
		uintptr(clsctxAll),
		uintptr(unsafe.Pointer(&iidIShellLinkW)),
		uintptr(unsafe.Pointer(&shellLink)),
	)
	if r != 0 {
		return fmt.Errorf("CoCreateInstance(IShellLinkW) failed (hresult 0x%x)", uint32(r))
	}
	sl := comPtr{p: shellLink}
	defer sl.release()

	// QueryInterface for IPersistFile (IUnknown index 0).
	var persist unsafe.Pointer
	r = sl.call(0, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&persist)))
	if r != 0 {
		return fmt.Errorf("QueryInterface(IPersistFile) failed (hresult 0x%x)", uint32(r))
	}
	pf := comPtr{p: persist}
	defer pf.release()

	exeDir := filepath.Dir(exePath)

	// IShellLinkW.SetPath (index 20).
	targetPtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return fmt.Errorf("encode target path: %w", err)
	}
	if r = sl.call(20, uintptr(unsafe.Pointer(targetPtr))); r != 0 {
		return fmt.Errorf("IShellLinkW.SetPath failed (hresult 0x%x)", uint32(r))
	}

	// IShellLinkW.SetWorkingDirectory (index 9).
	dirPtr, err := windows.UTF16PtrFromString(exeDir)
	if err != nil {
		return fmt.Errorf("encode working directory: %w", err)
	}
	if r = sl.call(9, uintptr(unsafe.Pointer(dirPtr))); r != 0 {
		return fmt.Errorf("IShellLinkW.SetWorkingDirectory failed (hresult 0x%x)", uint32(r))
	}

	// IShellLinkW.SetDescription (index 7).
	descPtr, err := windows.UTF16PtrFromString("Vepeen")
	if err != nil {
		return fmt.Errorf("encode description: %w", err)
	}
	if r = sl.call(7, uintptr(unsafe.Pointer(descPtr))); r != 0 {
		return fmt.Errorf("IShellLinkW.SetDescription failed (hresult 0x%x)", uint32(r))
	}

	// IShellLinkW.SetIconLocation (index 17): (LPCWSTR pszIconPath, int iIcon).
	iconPtr, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return fmt.Errorf("encode icon path: %w", err)
	}
	if r = sl.call(17, uintptr(unsafe.Pointer(iconPtr)), 0); r != 0 {
		return fmt.Errorf("IShellLinkW.SetIconLocation failed (hresult 0x%x)", uint32(r))
	}

	// IPersistFile.Save (index 6): (LPOLESTR pszFileName, BOOL fRemember).
	lnkPtr, err := windows.UTF16PtrFromString(lnkPath)
	if err != nil {
		return fmt.Errorf("encode lnk path: %w", err)
	}
	if r = pf.call(6, uintptr(unsafe.Pointer(lnkPtr)), 1); r != 0 {
		return fmt.Errorf("IPersistFile.Save failed (hresult 0x%x)", uint32(r))
	}

	return nil
}
