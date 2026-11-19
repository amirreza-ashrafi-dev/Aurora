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
	shell32    xwindows.Handle
	videoProc  uintptr
	dropProc   uintptr
	subclassID uintptr
	dropSubclassID uintptr

	dragAcceptFiles func(uintptr, int32)
	dragQueryFileW func(uintptr, uint32, *uint16, uint32) uint32
	dragFinish func(uintptr)

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
	shell, err := xwindows.LoadLibrary("shell32.dll")
	if err != nil {
		xwindows.FreeLibrary(comctl)
		xwindows.FreeLibrary(dll)
		return nil, err
	}
	p := &Platform{dll: dll, comctl32: comctl, shell32: shell, subclassID: 1, dropSubclassID: 2}
	h, ch, sh := uintptr(dll), uintptr(comctl), uintptr(shell)
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
	purego.RegisterLibFunc(&p.dragAcceptFiles, sh, "DragAcceptFiles")
	purego.RegisterLibFunc(&p.dragQueryFileW, sh, "DragQueryFileW")
	purego.RegisterLibFunc(&p.dragFinish, sh, "DragFinish")
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

// InstallVideoFileDrop enables shell file drops on the embedded video HWND.
// The callback is retained by Platform until RemoveVideoSubclass is called.
func (p *Platform) InstallVideoFileDrop(hwnd uintptr, onDrop func([]string)) error {
	if hwnd == 0 {
		return fmt.Errorf("video window is not initialized")
	}
	if p.dropProc != 0 {
		return fmt.Errorf("video file-drop handler is already installed")
	}
	const wmDropFiles uint32 = 0x0233
	callback := purego.NewCallback(func(window uintptr, message uint32, wParam uintptr, lParam uintptr, subclassID uintptr, refData uintptr) uintptr {
		if message != wmDropFiles {
			return p.defSubclassProc(window, message, wParam, lParam)
		}
		drop := wParam
		count := p.dragQueryFileW(drop, ^uint32(0), nil, 0)
		paths := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			length := p.dragQueryFileW(drop, i, nil, 0)
			if length == 0 {
				continue
			}
			buffer := make([]uint16, length+1)
			if p.dragQueryFileW(drop, i, &buffer[0], uint32(len(buffer))) != 0 {
				paths = append(paths, xwindows.UTF16ToString(buffer))
			}
		}
		p.dragFinish(drop)
		if onDrop != nil && len(paths) > 0 {
			onDrop(paths)
		}
		return 0
	})
	p.dragAcceptFiles(hwnd, 1)
	if p.setWindowSubclass(hwnd, callback, p.dropSubclassID, 0) == 0 {
		p.dragAcceptFiles(hwnd, 0)
		return fmt.Errorf("SetWindowSubclass for file drops failed")
	}
	p.dropProc = callback
	return nil
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
	if p == nil || hwnd == 0 {
		return
	}
	if p.videoProc != 0 {
		p.removeWindowSubclass(hwnd, p.videoProc, p.subclassID)
		p.videoProc = 0
	}
	if p.dropProc != 0 {
		p.removeWindowSubclass(hwnd, p.dropProc, p.dropSubclassID)
		p.dragAcceptFiles(hwnd, 0)
		p.dropProc = 0
	}
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
	if p.comctl32 != 0 {
		xwindows.FreeLibrary(p.comctl32)
		p.comctl32 = 0
	}
	if p.shell32 != 0 {
		xwindows.FreeLibrary(p.shell32)
		p.shell32 = 0
	}
	if p.dll != 0 {
		xwindows.FreeLibrary(p.dll)
		p.dll = 0
	}
}
