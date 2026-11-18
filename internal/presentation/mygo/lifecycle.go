package presentation

// Close releases all native and playback resources. It is safe to call once
// the MyGo event loop has returned, even after partial initialization.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.stopControllerMonitor()
		a.stopPolling()
		a.setFullscreenCursorHidden(false)
		if a.controller != nil {
			if !a.controller.IsDestroyed() {
				a.controller.Destroy()
			}
			a.controller = nil
		}
		if a.player != nil {
			a.player.Close()
			a.player = nil
		}
		if a.videoHWND != 0 && a.platform != nil {
			a.platform.RemoveVideoSubclass(a.videoHWND)
			a.platform.DestroyWindow(a.videoHWND)
			a.videoHWND = 0
		}
		if a.platform != nil {
			a.platform.Close()
			a.platform = nil
		}
	})
}
