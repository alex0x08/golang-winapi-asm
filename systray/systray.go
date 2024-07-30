package systray

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file contains all systray logic
// @author alex0x08

var (
	// a simple callback to handle clicks on systray
	trayClickCallback func()
)

// see https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-app
const TrayIconMsg = WM_APP + 1

// some internal types to make life easier
type (
	HANDLE uintptr
	HICON  HANDLE
	HWND   HANDLE

	TrayIcon struct {
		hwnd uintptr
		guid GUID
	}
)

// Predefined window handles
const (
	// see https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features#message-only-windows
	HWND_MESSAGE = ^HWND(2) // -3
)

// creates systray icon
func CreateSysTray() *TrayIcon {
	// first, create hidden message-only window
	hwnd, err := createMessageWindow()
	if err != nil {
		panic(err)
	}
	// create systray with parent = our message-only window
	ti, err := newTrayIcon(hwnd)
	if err != nil {
		panic(err)
	}
	return ti
}

func SetTrayClickHandler(fn func()) {
	trayClickCallback = fn
}

func (ti *TrayIcon) Dispose() error {
	_, err := Shell_NotifyIcon(NIM_DELETE, ti.initData())
	return err
}

func (ti *TrayIcon) SetIcon(icon uintptr) error {
	data := ti.initData()
	data.UFlags |= NIF_ICON
	data.HIcon = icon
	_, err := Shell_NotifyIcon(NIM_MODIFY, data)
	return err
}

func (ti *TrayIcon) SetTooltip(tooltip string) error {
	data := ti.initData()
	data.UFlags |= NIF_TIP
	copy(data.SzTip[:], windows.StringToUTF16(tooltip))
	_, err := Shell_NotifyIcon(NIM_MODIFY, data)
	return err
}

func createMessageWindow() (uintptr, error) {
	hInstance, err := GetModuleHandle(nil)
	if err != nil {
		return 0, err
	}

	wndClass := windows.StringToUTF16Ptr("MyWindow")

	var wcex WNDCLASSEX

	wcex.CbSize = uint32(unsafe.Sizeof(wcex))
	wcex.LpfnWndProc = windows.NewCallback(wndProc)
	wcex.HInstance = hInstance
	wcex.LpszClassName = wndClass
	if _, err := RegisterClassEx(&wcex); err != nil {
		return 0, err
	}

	hwnd, err := CreateWindowEx(
		0,
		wndClass,
		windows.StringToUTF16Ptr(""),
		WS_OVERLAPPED,
		CW_USEDEFAULT,
		CW_USEDEFAULT,
		400,
		300,
		uintptr(HWND_MESSAGE),
		0,
		hInstance,
		nil)
	if err != nil {
		return 0, err
	}
	return hwnd, nil
}

// this is main window function
// see https://learn.microsoft.com/en-us/windows/win32/api/winuser/nc-winuser-wndproc
func wndProc(hWnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case TrayIconMsg:
		nmsg := LOWORD(uint32(lParam))
		// if user clicked on tray icon
		if nmsg == WM_LBUTTONDOWN {
			// if callback function exist
			if trayClickCallback != nil {
				trayClickCallback()
			}

		}
	case WM_DESTROY:
		PostQuitMessage(0)
	default:
		r, _ := DefWindowProc(hWnd, msg, wParam, lParam)
		return r
	}
	return 0
}

func newTrayIcon(hwnd uintptr) (*TrayIcon, error) {
	guid2, _ := windows.GenerateGUID()
	ti := &TrayIcon{hwnd: hwnd, guid: GUID(guid2)}
	data := ti.initData()
	data.UFlags |= NIF_MESSAGE
	data.UCallbackMessage = TrayIconMsg
	if _, err := Shell_NotifyIcon(NIM_ADD, data); err != nil {
		return nil, err
	}
	return ti, nil
}

func (ti *TrayIcon) initData() *NOTIFYICONDATA {
	var data NOTIFYICONDATA
	data.CbSize = uint32(unsafe.Sizeof(data))
	data.UFlags = NIF_GUID
	data.HWnd = ti.hwnd
	data.GUIDItem = ti.guid
	return &data
}
