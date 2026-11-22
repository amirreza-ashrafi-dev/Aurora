//go:build darwin

package videohost

import (
	"errors"
	"fmt"
	"sync"
	"structs"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	// Importing native installs MyGo's ui.NativeView embedding hook. The
	// host itself is an ordinary Go-owned NSView, not a MyGo native module.
	_ "github.com/egoist/mygo/native"
)

const appKitPath = "/System/Library/Frameworks/AppKit.framework/AppKit"

var (
	appKitOnce   sync.Once
	appKitHandle uintptr
	appKitErr    error

	selAlloc                       = objc.RegisterName("alloc")
	selInitWithFrame               = objc.RegisterName("initWithFrame:")
	selSetWantsLayer               = objc.RegisterName("setWantsLayer:")
	selSetPostsBoundsNotifications = objc.RegisterName("setPostsBoundsChangedNotifications:")
	selSetPostsFrameNotifications  = objc.RegisterName("setPostsFrameChangedNotifications:")
	selRemoveFromSuperview         = objc.RegisterName("removeFromSuperview")
	selRelease                     = objc.RegisterName("release")
)

// nsRect has the same ABI layout as AppKit's NSRect on 64-bit macOS.
type nsRect struct {
	_      structs.HostLayout
	X, Y   float64
	Width  float64
	Height float64
}

// VideoHost owns the NSView that MyGo places in the video stage.
// MPV's patched macvk output adds its own Metal-backed view below this host.
type VideoHost struct {
	mu        sync.Mutex
	view      objc.ID
	closeOnce sync.Once
}

func loadAppKit() error {
	appKitOnce.Do(func() {
		appKitHandle, appKitErr = purego.Dlopen(appKitPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if appKitErr != nil {
			appKitErr = fmt.Errorf("load AppKit: %w", appKitErr)
		}
	})
	return appKitErr
}

// New creates an empty layer-backed NSView. It is called from MyGo's
// application-ready callback, which runs on the AppKit main thread.
func New() (*VideoHost, error) {
	if err := loadAppKit(); err != nil {
		return nil, err
	}

	nsViewClass := objc.GetClass("NSView")
	if nsViewClass == 0 {
		return nil, errors.New("AppKit did not register NSView")
	}

	view := objc.ID(nsViewClass).Send(selAlloc)
	if view == 0 {
		return nil, errors.New("allocate NSView")
	}

	view = view.Send(selInitWithFrame, nsRect{})
	if view == 0 {
		return nil, errors.New("initialize NSView")
	}

	// MPV observes these notifications to keep its drawable and Vulkan
	// swapchain in sync with MyGo's layout and window changes.
	view.Send(selSetWantsLayer, uintptr(1))
	view.Send(selSetPostsBoundsNotifications, uintptr(1))
	view.Send(selSetPostsFrameNotifications, uintptr(1))

	return &VideoHost{view: view}, nil
}

// NativeView is the NSView* consumed by ui.NativeView and libmpv's wid option.
func (h *VideoHost) NativeView() uintptr {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return uintptr(h.view)
}

// Close releases Aurora's ownership after the libmpv player has detached.
// MyGo separately releases the retain it takes while embedding the view.
func (h *VideoHost) Close() error {
	if h == nil {
		return nil
	}
	h.closeOnce.Do(func() {
		h.mu.Lock()
		view := h.view
		h.view = 0
		h.mu.Unlock()

		if view != 0 {
			view.Send(selRemoveFromSuperview)
			view.Send(selRelease)
		}
	})
	return nil
}
