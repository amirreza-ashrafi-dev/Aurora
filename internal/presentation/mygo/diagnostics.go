package presentation

import "log"

func (a *App) logDiagnostics(label string) {
	if a == nil {
		return
	}
	if a.win != nil && !a.win.IsDestroyed() {
		cb := a.win.ContentBounds()
		log.Printf("[diag] %s main mygo ContentBounds (DIPs) = %+v", label, cb)
		if a.platform != nil {
			a.platform.LogHWND(label+" main", a.win.NativeHandle())
		}
	} else {
		log.Printf("[diag] %s main window is nil or destroyed", label)
	}
	if a.controller != nil && !a.controller.IsDestroyed() {
		cb := a.controller.ContentBounds()
		log.Printf("[diag] %s controller mygo ContentBounds (DIPs) = %+v", label, cb)
		if a.platform != nil {
			a.platform.LogHWND(label+" controller", a.controller.NativeHandle())
		}
	} else {
		log.Printf("[diag] %s controller is nil or destroyed", label)
	}
}
