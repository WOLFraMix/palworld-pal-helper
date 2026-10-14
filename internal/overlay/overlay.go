//go:build windows

package overlay

import (
	"errors"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	procGlobalSize       = kernel32.NewProc("GlobalSize")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")

	// GDI — рисование текста и примитивов
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
)

const (
	WS_POPUP           = 0x80000000
	WS_EX_TOPMOST      = 0x00000008
	WS_EX_TOOLWINDOW   = 0x00000080
	WS_EX_NOACTIVATE   = 0x08000000
	SW_SHOWNOACTIVATE  = 4
	SW_HIDE            = 0
	HWND_TOPMOST       = ^uintptr(0) // -1
	SWP_NOSIZE         = 0x0001
	SWP_NOMOVE         = 0x0002
	SWP_NOACTIVATE     = 0x0010
	SWP_SHOWWINDOW     = 0x0040
	WM_DESTROY         = 0x0002
	WM_PAINT           = 0x000F
	WM_TIMER           = 0x0113
	WM_HOTKEY          = 0x0312
	WM_LBUTTONDOWN     = 0x0201
	MOD_ALT            = 0x0001
	MOD_CONTROL        = 0x0002
	VK_Q               = 0x51
	CF_UNICODETEXT     = 13
	DT_LEFT            = 0x0000
	DT_WORDBREAK       = 0x0010
	DT_NOPREFIX        = 0x0800
	DT_END_ELLIPSIS    = 0x8000
	TRANSPARENT_BKMODE = 1
	DEFAULT_CHARSET    = 1
	CLEARTYPE_QUALITY  = 5
	WS_EX_LAYERED      = 0x00080000
	LWA_ALPHA          = 0x00000002
)

const winW, winH, hideAfterMs = 640, 240, 5000

type wndclassex struct {
	CbSize, Style                            uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground uintptr
	LpszMenuName, LpszClassName, HIconSm     *uint16
}

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}

type paintstruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rect
	FRestore    int32
	FIncUpdate  int32
	FIncUpdateP int32
	RgbReserved [32]byte
}

var (
	searchFn  func(string) string
	currentTx string
)

// Run запускает цикл сообщений и не возвращается до выхода.
// search — колбэк: получает текст из буфера обмена, возвращает текст для панели.
func Run(search func(string) string) error {
	if search == nil {
		return nil
	}
	searchFn = search

	hInst, _, _ := procGetModuleHandleW.Call(0)
	cn, _ := syscall.UTF16PtrFromString("PalHelperOverlay")

	wc := wndclassex{
		CbSize:        uint32(unsafe.Sizeof(wndclassex{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInst,
		LpszClassName: cn,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	h, _, _ := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_LAYERED,
		uintptr(unsafe.Pointer(cn)), uintptr(unsafe.Pointer(cn)),
		WS_POPUP, 100, 100, winW, winH, 0, 0, hInst, 0)
	if h == 0 {
		return errors.New("CreateWindowExW не сработал")
	}
	// полупрозрачность панели: 235 из 255 — чуть просвечивает фон
	procSetLayeredWindowAttributes.Call(h, 0, 200, LWA_ALPHA)

	procRegisterHotKey.Call(h, 1, MOD_CONTROL|MOD_ALT, VK_Q)

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int64(r) <= 0 {
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(h, m, wp, lp uintptr) uintptr {
	switch m {
	case WM_HOTKEY:
		onHotkey(h)
		return 0
	case WM_TIMER:
		procKillTimer.Call(h, 1)
		procShowWindow.Call(h, SW_HIDE)
		return 0
	case WM_LBUTTONDOWN: // клик по панели — убрать
		procShowWindow.Call(h, SW_HIDE)
		return 0
	case WM_PAINT:
		paint(h)
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(h, m, wp, lp)
	return r
}

func onHotkey(h uintptr) {
	q := strings.TrimSpace(readClipboard())
	if q == "" {
		return
	}
	tx := searchFn(q)
	if tx == "" {
		return
	}
	currentTx = tx

	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	x := (int(sw) - winW) / 2
	y := int(sh) - winH - 90

	procSetWindowPos.Call(h, HWND_TOPMOST, uintptr(x), uintptr(y),
		0, 0, SWP_NOSIZE|SWP_NOACTIVATE|SWP_SHOWWINDOW)
	procShowWindow.Call(h, SW_SHOWNOACTIVATE)
	procInvalidateRect.Call(h, 0, 0)
	procSetTimer.Call(h, 1, hideAfterMs, 0)
}

func paint(h uintptr) {
	var ps paintstruct
	hdc, _, _ := procBeginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(h, uintptr(unsafe.Pointer(&ps)))

	bg, _, _ := procCreateSolidBrush.Call(0x001F1F1F) // BGR: тёмно-серый
	defer procDeleteObject.Call(bg)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&ps.RcPaint)), bg)

	if currentTx == "" {
		return
	}

	// высота шрифта отрицательная — так Win32 понимает «высоту символа»
	fontH := int32(-22)
	font, _, _ := procCreateFontW.Call(
		uintptr(fontH), 0, 0, 0, 600, 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, 0,
		uintptr(unsafe.Pointer(mustPtr("Segoe UI"))))
	defer procDeleteObject.Call(font)
	old, _, _ := procSelectObject.Call(hdc, font)
	defer procSelectObject.Call(hdc, old)

	procSetBkMode.Call(hdc, TRANSPARENT_BKMODE)
	procSetTextColor.Call(hdc, 0x00F0E0C0) // COLORREF 0x00BBGGRR — бежево-голубой

	u, _ := syscall.UTF16FromString(currentTx)
	rc := rect{12, 10, winW - 12, winH - 10}
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])),
		uintptr(len(u)-1), uintptr(unsafe.Pointer(&rc)),
		DT_LEFT|DT_WORDBREAK|DT_NOPREFIX|DT_END_ELLIPSIS)
}

func readClipboard() string {
	if r, _, _ := procOpenClipboard.Call(0); r == 0 {
		return ""
	}
	defer procCloseClipboard.Call()
	hData, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if hData == 0 {
		return ""
	}
	p, _, _ := procGlobalLock.Call(hData)
	if p == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(hData)

	n, _, _ := procGlobalSize.Call(hData)
	if n < 2 {
		return ""
	}
	u := unsafe.Slice((*uint16)(unsafe.Pointer(p)), n/2)
	for i, c := range u {
		if c == 0 { // нулевой терминатор — конец строки
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u))
}

func mustPtr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}
