package videohost

import (
	"fmt"

	"github.com/egoist/mygo/native"
)

// Host is the NSView created by the native module and mounted by MyGo.
//mygo:native
type Host interface {
	native.View
}

// VideoHost is the application-facing owner of the native view.
type VideoHost interface {
	Host
	Close() error
}

type ownedHost struct {
	Host
	object *native.Object
}

// New creates a MyGo-owned NSView and keeps its native object handle so the
// application can release it deterministically after MPV has detached.
func New() (VideoHost, error) {
	host, err := NewHost()
	if err != nil {
		return nil, err
	}
	objectProvider, ok := host.(interface{ NativeObject() *native.Object })
	if !ok || objectProvider.NativeObject() == nil {
		return nil, fmt.Errorf("MyGo native VideoHost did not expose its object handle")
	}
	return &ownedHost{Host: host, object: objectProvider.NativeObject()}, nil
}

func (h *ownedHost) Close() error {
	if h == nil || h.object == nil {
		return nil
	}
	err := h.object.Close()
	h.object = nil
	return err
}
