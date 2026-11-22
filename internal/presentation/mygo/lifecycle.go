package presentation

// Close releases all native and playback resources. It is safe to call once
// the MyGo event loop has returned, even after partial initialization.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		if a.startupFilesTimer != nil {
			a.startupFilesTimer.Stop()
			a.startupFilesTimer = nil
		}
		a.closeSubtitleFlyout()
		a.closeFullscreenSubtitleFlyout()
		a.closeResumeBanner()
		a.closeVolumeOSD()
		a.abandonFilmstripAfterShutdown()
		a.stopMemoryMonitor()
		if a.memoryWindow != nil && !a.memoryWindow.IsDestroyed() {
			a.memoryWindow.Destroy()
			a.memoryWindow = nil
		}
		if a.fullscreenHeader != nil && !a.fullscreenHeader.IsDestroyed() {
			a.fullscreenHeader.Destroy()
			a.fullscreenHeader = nil
		}
		a.stopControllerMonitor()
		a.stopPolling()
		a.saveCurrentProgressImmediately()
		a.stopProgressWorker()
		a.setFullscreenCursorHidden(false)
		if a.controller != nil {
			if !a.controller.IsDestroyed() {
				a.controller.Destroy()
			}
			a.controller = nil
		}
		a.closePlayer()
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
