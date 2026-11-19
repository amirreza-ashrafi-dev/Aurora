//go:build windows

package windows

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	xwindows "golang.org/x/sys/windows"

	"my-go/internal/domain"
)

type rect struct{ Left, Top, Right, Bottom int32 }
type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

const (
	gwOwner                 = 4
	monitorDefaultToNearest = 2
)

// Platform owns the Win32 APIs used to host libmpv's child video surface.
// All MyGo-specific geometry conversion happens at the presentation boundary.
type Platform struct {
	dll        xwindows.Handle
	comctl32   xwindows.Handle
	kernel32   xwindows.Handle
	videoProc  uintptr
	subclassID uintptr

	createWindowEx       func(uint32, uintptr, uintptr, uint32, int32, int32, int32, int32, uintptr, uintptr, uintptr, uintptr) uintptr
	showWindow           func(uintptr, int32) int32
	updateWindow         func(uintptr) int32
	destroyWindow        func(uintptr) int32
	getClientRect        func(uintptr, *rect) int32
	getWindowRect        func(uintptr, *rect) int32
	getCursorPos         func(*domain.Point) int32
	showCursor           func(int32) int32
	setWindowPos         func(uintptr, uintptr, int32, int32, int32, int32, uint32) int32
	getWindowLongPtr     func(uintptr, int32) uintptr
	setWindowLongPtr     func(uintptr, int32, uintptr) uintptr
	setWindowSubclass    func(uintptr, uintptr, uintptr, uintptr) int32
	removeWindowSubclass func(uintptr, uintptr, uintptr) int32
	defSubclassProc      func(uintptr, uint32, uintptr, uintptr) uintptr
	getParent            func(uintptr) uintptr
	getWindow            func(uintptr, uint32) uintptr
	monitorFromWindow    func(uintptr, uint32) uintptr
	getMonitorInfoW      func(uintptr, *monitorInfo) int32
	getDpiForWindow      func(uintptr) uint32
	clientToScreen      func(uintptr, *domain.Point) int32
	getModuleHandle     func(uintptr) uintptr
	setWindowsHookEx    func(int32, uintptr, uintptr, uint32) uintptr
	unhookWindowsHookEx func(uintptr) int32
	callNextHookEx      func(uintptr, int32, uintptr, uintptr) uintptr

	mouseHook     uintptr
	mouseHookProc uintptr
	mouseHookFunc func(int32, uintptr, uintptr) uintptr

	outsidePopup       uintptr
	outsideAnchorWindow uintptr
	outsideAnchor      domain.Rect
	onOutsideClick     func()
}

// New loads the Win32 and common-controls APIs through purego.
func New() (*Platform, error) {
	dll, err := xwindows.LoadLibrary("user32.dll")
	if err != nil {
		return nil, err
	}
	comctl, err := xwindows.LoadLibrary("comctl32.dll")
	if err != nil {
		xwindows.FreeLibrary(dll)
		return nil, err
	}
	kernel32, err := xwindows.LoadLibrary("kernel32.dll")
	if err != nil {
		xwindows.FreeLibrary(comctl)
		xwindows.FreeLibrary(dll)
		return nil, err
	}

	p := &Platform{dll: dll, comctl32: comctl, kernel32: kernel32, subclassID: 1}
	h, ch, kh := uintptr(dll), uintptr(comctl), uintptr(kernel32)
	purego.RegisterLibFunc(&p.createWindowEx, h, "CreateWindowExW")
	purego.RegisterLibFunc(&p.showWindow, h, "ShowWindow")
	purego.RegisterLibFunc(&p.updateWindow, h, "UpdateWindow")
	purego.RegisterLibFunc(&p.destroyWindow, h, "DestroyWindow")
	purego.RegisterLibFunc(&p.getClientRect, h, "GetClientRect")
	purego.RegisterLibFunc(&p.getWindowRect, h, "GetWindowRect")
	purego.RegisterLibFunc(&p.getCursorPos, h, "GetCursorPos")
	purego.RegisterLibFunc(&p.showCursor, h, "ShowCursor")
	purego.RegisterLibFunc(&p.setWindowPos, h, "SetWindowPos")
	purego.RegisterLibFunc(&p.getWindowLongPtr, h, "GetWindowLongPtrW")
	purego.RegisterLibFunc(&p.setWindowLongPtr, h, "SetWindowLongPtrW")
	purego.RegisterLibFunc(&p.setWindowSubclass, ch, "SetWindowSubclass")
	purego.RegisterLibFunc(&p.removeWindowSubclass, ch, "RemoveWindowSubclass")
	purego.RegisterLibFunc(&p.defSubclassProc, ch, "DefSubclassProc")
	purego.RegisterLibFunc(&p.getParent, h, "GetParent")
	purego.RegisterLibFunc(&p.getWindow, h, "GetWindow")
	purego.RegisterLibFunc(&p.monitorFromWindow, h, "MonitorFromWindow")
	purego.RegisterLibFunc(&p.getMonitorInfoW, h, "GetMonitorInfoW")
	purego.RegisterLibFunc(&p.getDpiForWindow, h, "GetDpiForWindow")
	purego.RegisterLibFunc(&p.clientToScreen, h, "ClientToScreen")
	// GetModuleHandleW is exported by kernel32.dll, not user32.dll.
	purego.RegisterLibFunc(&p.getModuleHandle, kh, "GetModuleHandleW")
	purego.RegisterLibFunc(&p.setWindowsHookEx, h, "SetWindowsHookExW")
	purego.RegisterLibFunc(&p.unhookWindowsHookEx, h, "UnhookWindowsHookEx")
	purego.RegisterLibFunc(&p.callNextHookEx, h, "CallNextHookEx")
	return p, nil
}

func (p *Platform) CreateVideoHost(parent uintptr) (uintptr, error) {
	className, err := xwindows.UTF16PtrFromString("STATIC")
	if err != nil {
		return 0, err
	}
	title, err := xwindows.UTF16PtrFromString("Aurora Video")
	if err != nil {
		return 0, err
	}
	const style uint32 = 0x40000000 | 0x04000000 | 0x02000000 | 0x00000004 | 0x00000100
	hwnd := p.createWindowEx(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), style, 0, 0, 1, 1, parent, 0, 0, 0)
	runtime.KeepAlive(className)
	runtime.KeepAlive(title)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed")
	}
	p.showWindow(hwnd, 0) // SW_HIDE: preserve the MyGo empty state until media is loaded.
	return hwnd, nil
}

func (p *Platform) InstallVideoDoubleClick(hwnd uintptr, onDoubleClick func()) error {
	if hwnd == 0 {
		return fmt.Errorf("video window is not initialized")
	}
	const wmLButtonDblClk uint32 = 0x0203
	callback := purego.NewCallback(func(window uintptr, message uint32, wParam uintptr, lParam uintptr, subclassID uintptr, refData uintptr) uintptr {
		if message == wmLButtonDblClk && onDoubleClick != nil {
			onDoubleClick()
		}
		return p.defSubclassProc(window, message, wParam, lParam)
	})
	if p.setWindowSubclass(hwnd, callback, p.subclassID, 0) == 0 {
		return fmt.Errorf("SetWindowSubclass failed")
	}
	p.videoProc = callback
	return nil
}

const (
	whMouseLL        = 14
	wmLButtonDown    = 0x0201
	wmRButtonDown    = 0x0204
	wmMButtonDown    = 0x0207
	wmXButtonDown    = 0x020B
)

// mouseHookInfo is MSLLHOOKSTRUCT as Win32 lays it out on 64-bit Windows.
type mouseHookInfo struct {
	Point     domain.Point
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// WatchOutsideClickDismiss dismisses an open native popup when a mouse button
// is pressed outside both the popup and its CC anchor button. The low-level
// hook also sees clicks delivered to MPV's child HWND, which are invisible
// to MyGo's parent-window UI event handlers.
func (p *Platform) WatchOutsideClickDismiss(popupHWND, anchorWindow uintptr, anchor domain.Rect, onOutside func()) error {
	if p == nil || popupHWND == 0 || anchorWindow == 0 || onOutside == nil {
		return fmt.Errorf("outside-click dismissal requires a popup, anchor window, and callback")
	}
	p.StopOutsideClickDismiss()
	p.outsidePopup = popupHWND
	p.outsideAnchorWindow = anchorWindow
	p.outsideAnchor = anchor
	p.onOutsideClick = onOutside
	p.mouseHookFunc = func(code int32, message uintptr, data uintptr) uintptr {
		if code >= 0 && data != 0 && isMouseButtonDown(message) {
			click := (*mouseHookInfo)(unsafe.Pointer(data)).Point
			if !p.pointInWindow(click, p.outsidePopup) && !p.pointInAnchor(click) {
				callback := p.onOutsideClick
				if callback != nil {
					callback()
				}
			}
		}
		if p.callNextHookEx != nil {
			return p.callNextHookEx(0, code, message, data)
		}
		return 0
	}
	p.mouseHookProc = purego.NewCallback(p.mouseHookFunc)
	// A zero thread ID requests a desktop-wide low-level mouse hook;
	// pass the current executable module containing the Go callback.
	module := p.getModuleHandle(0)
	p.mouseHook = p.setWindowsHookEx(whMouseLL, p.mouseHookProc, module, 0)
	if p.mouseHook == 0 {
		p.onOutsideClick = nil
		p.mouseHookFunc = nil
		p.mouseHookProc = 0
		p.outsidePopup = 0
		p.outsideAnchorWindow = 0
		return fmt.Errorf("SetWindowsHookExW(WH_MOUSE_LL) failed")
	}
	return nil
}

// UpdateOutsideClickDismissAnchor follows the CC button if the controller
// layout changes while the popup remains open.
func (p *Platform) UpdateOutsideClickDismissAnchor(anchorWindow uintptr, anchor domain.Rect) {
	if p == nil || p.mouseHook == 0 || anchorWindow == 0 {
		return
	}
	p.outsideAnchorWindow = anchorWindow
	p.outsideAnchor = anchor
}

func (p *Platform) StopOutsideClickDismiss() {
	if p == nil {
		return
	}
	if p.mouseHook != 0 && p.unhookWindowsHookEx != nil {
		p.unhookWindowsHookEx(p.mouseHook)
	}
	p.mouseHook = 0
	p.mouseHookProc = 0
	p.mouseHookFunc = nil
	p.onOutsideClick = nil
	p.outsidePopup = 0
	p.outsideAnchorWindow = 0
	p.outsideAnchor = domain.Rect{}
}

func isMouseButtonDown(message uintptr) bool {
	switch message {
	case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown:
		return true
	default:
		return false
	}
}

func (p *Platform) pointInWindow(point domain.Point, hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	var bounds rect
	if p.getWindowRect(hwnd, &bounds) == 0 {
		return false
	}
	return point.X >= bounds.Left && point.Y >= bounds.Top &&
		point.X < bounds.Right && point.Y < bounds.Bottom
}

func (p *Platform) pointInAnchor(point domain.Point) bool {
	hwnd := p.outsideAnchorWindow
	if hwnd == 0 {
		return false
	}
	var origin domain.Point
	if p.clientToScreen(hwnd, &origin) == 0 {
		return false
	}
	dpi := p.WindowDPI(hwnd)
	scale := float64(dpi) / 96.0
	anchor := p.outsideAnchor
	left := origin.X + int32(math.Round(float64(anchor.X)*scale))
	top := origin.Y + int32(math.Round(float64(anchor.Y)*scale))
	right := left + int32(math.Round(float64(anchor.W)*scale))
	bottom := top + int32(math.Round(float64(anchor.H)*scale))
	return point.X >= left && point.Y >= top && point.X < right && point.Y < bottom
}

func (p *Platform) LayoutVideo(parent, child uintptr, fullscreen bool, stage domain.Rect) error {
	if parent == 0 || child == 0 {
		return nil
	}
	var client rect
	if p.getClientRect(parent, &client) == 0 {
		return fmt.Errorf("GetClientRect failed")
	}
	var x, y, width, height int32
	if fullscreen {
		width, height = client.Right-client.Left, client.Bottom-client.Top
	} else {
		if stage.W <= 0 || stage.H <= 0 {
			return nil
		}
		dpi := p.WindowDPI(parent)
		scale := float64(dpi) / 96.0
		toPixels := func(value float32) int32 { return int32(math.Round(float64(value) * scale)) }
		x, y, width, height = toPixels(stage.X), toPixels(stage.Y), toPixels(stage.W), toPixels(stage.H)
	}
	if width < 1 || height < 1 {
		return nil
	}
	const flags uint32 = 0x0010 | 0x0004 // SWP_NOACTIVATE | SWP_NOZORDER
	if p.setWindowPos(child, 0, x, y, width, height, flags) == 0 {
		return fmt.Errorf("SetWindowPos for video stage failed")
	}
	return nil
}

func (p *Platform) CursorInsideWindow(hwnd uintptr) (bool, domain.Point) {
	var point domain.Point
	var bounds rect
	if p.getCursorPos(&point) == 0 || p.getWindowRect(hwnd, &bounds) == 0 {
		return false, point
	}
	inside := point.X >= bounds.Left && point.Y >= bounds.Top && point.X < bounds.Right && point.Y < bounds.Bottom
	return inside, point
}

func (p *Platform) WindowDPI(hwnd uintptr) uint32 {
	if p == nil || p.getDpiForWindow == nil {
		return 96
	}
	if dpi := p.getDpiForWindow(hwnd); dpi != 0 {
		return dpi
	}
	return 96
}

func (p *Platform) ShowCursor(show int32) int32                  { return p.showCursor(show) }
func (p *Platform) ShowWindow(hwnd uintptr, command int32) int32 { return p.showWindow(hwnd, command) }
func (p *Platform) UpdateWindow(hwnd uintptr) int32              { return p.updateWindow(hwnd) }
func (p *Platform) DestroyWindow(hwnd uintptr) int32             { return p.destroyWindow(hwnd) }

// RevealChild restores the embedded video HWND to the top of child z-order
// without changing focus or its current geometry.
func (p *Platform) RevealChild(hwnd uintptr) error {
	const (
		swpNoSize        = uint32(0x0001)
		swpNoMove        = uint32(0x0002)
		swpNoActivate    = uint32(0x0010)
		swpShowWindow    = uint32(0x0040)
		swShowNoActivate = int32(4)
	)
	if p.setWindowPos(hwnd, 0, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate|swpShowWindow) == 0 {
		return fmt.Errorf("SetWindowPos(SWP_SHOWWINDOW) failed")
	}
	p.showWindow(hwnd, swShowNoActivate)
	p.updateWindow(hwnd)
	return nil
}

func (p *Platform) RemoveVideoSubclass(hwnd uintptr) {
	if p == nil || hwnd == 0 || p.videoProc == 0 {
		return
	}
	p.removeWindowSubclass(hwnd, p.videoProc, p.subclassID)
	p.videoProc = 0
}

func (p *Platform) LogHWND(label string, hwnd uintptr) {
	if p == nil || hwnd == 0 {
		log.Printf("[diag] %s: hwnd is 0", label)
		return
	}
	parent := p.getParent(hwnd)
	owner := p.getWindow(hwnd, gwOwner)
	dpi := p.WindowDPI(hwnd)
	var clientRect, windowRect rect
	p.getClientRect(hwnd, &clientRect)
	p.getWindowRect(hwnd, &windowRect)
	monitor := p.monitorFromWindow(hwnd, monitorDefaultToNearest)
	var mi monitorInfo
	mi.Size = uint32(unsafe.Sizeof(mi))
	haveMonitor := p.getMonitorInfoW(monitor, &mi) != 0
	log.Printf("[diag] %s hwnd=%#x parent=%#x owner=%#x dpi=%d (%.0f%%)", label, hwnd, parent, owner, dpi, float64(dpi)/96.0*100)
	log.Printf("    client rect (local, physical px)  = (%d,%d)-(%d,%d) size=%dx%d", clientRect.Left, clientRect.Top, clientRect.Right, clientRect.Bottom, clientRect.Right-clientRect.Left, clientRect.Bottom-clientRect.Top)
	log.Printf("    window rect (screen, physical px) = (%d,%d)-(%d,%d) size=%dx%d", windowRect.Left, windowRect.Top, windowRect.Right, windowRect.Bottom, windowRect.Right-windowRect.Left, windowRect.Bottom-windowRect.Top)
	if haveMonitor {
		log.Printf("    monitor=%#x rect=(%d,%d)-(%d,%d) work=(%d,%d)-(%d,%d) primary=%v", monitor, mi.Monitor.Left, mi.Monitor.Top, mi.Monitor.Right, mi.Monitor.Bottom, mi.Work.Left, mi.Work.Top, mi.Work.Right, mi.Work.Bottom, mi.Flags&1 != 0)
	} else {
		log.Printf("    GetMonitorInfoW failed for monitor=%#x", monitor)
	}
}

// MakeControllerNonActivating is retained for diagnostics and future native-window experiments.
func (p *Platform) MakeControllerNonActivating(hwnd uintptr) error {
	if hwnd == 0 {
		return fmt.Errorf("controller HWND is not initialized")
	}
	const (
		gwlExStyle      = -20
		wsExToolWindow  = uintptr(0x00000080)
		wsExNoActivate  = uintptr(0x08000000)
		swpNoSize       = uint32(0x0001)
		swpNoMove       = uint32(0x0002)
		swpNoZOrder     = uint32(0x0004)
		swpNoActivate   = uint32(0x0010)
		swpFrameChanged = uint32(0x0020)
	)
	style := p.getWindowLongPtr(hwnd, gwlExStyle) | wsExToolWindow | wsExNoActivate
	p.setWindowLongPtr(hwnd, gwlExStyle, style)
	flags := swpNoMove | swpNoSize | swpNoZOrder | swpNoActivate | swpFrameChanged
	if p.setWindowPos(hwnd, 0, 0, 0, 0, 0, flags) == 0 {
		return fmt.Errorf("SetWindowPos for controller styles failed")
	}
	return nil
}

func (p *Platform) Close() {
	if p == nil {
		return
	}
	p.StopOutsideClickDismiss()
	if p.comctl32 != 0 {
		xwindows.FreeLibrary(p.comctl32)
		p.comctl32 = 0
	}
	if p.dll != 0 {
		xwindows.FreeLibrary(p.dll)
		p.dll = 0
	}
	if p.kernel32 != 0 {
		xwindows.FreeLibrary(p.kernel32)
		p.kernel32 = 0
	}
}
