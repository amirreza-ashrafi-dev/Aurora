package presentation

import "my-go/internal/application"

// SetStartupFiles supplies videos passed to Aurora by Windows Explorer. The
// files are loaded after the native video surface and main window are ready.
func (a *App) SetStartupFiles(paths []string) {
	if a == nil {
		return
	}
	a.startupFiles = application.FilterVideoLaunchPaths(paths)
}
