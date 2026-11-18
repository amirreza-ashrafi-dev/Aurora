package presentation

import (
	"log"

	"github.com/egoist/mygo/ui"

	"my-go/internal/domain"
)

func (a *App) layoutVideo() {
	if a.platform == nil || a.win == nil || a.videoHWND == 0 {
		return
	}
	if !a.fullscreen && !a.hasVideoStageBounds {
		return
	}
	if err := a.platform.LayoutVideo(a.win.NativeHandle(), a.videoHWND, a.fullscreen, a.videoStageBounds); err != nil {
		log.Println("video layout:", err)
	}
}

// syncVideoHostToStage applies the committed MyGo stage bounds to the native
// MPV child HWND, keeping the Windows adapter independent of MyGo UI types.
func (a *App) syncVideoHostToStage(c *ui.Context) {
	if a == nil || c == nil || a.platform == nil || a.win == nil || a.videoHWND == 0 {
		return
	}
	bounds := a.videoStageHandle.Bounds(c)
	if bounds.W <= 0 || bounds.H <= 0 {
		return
	}
	dpi := a.platform.WindowDPI(a.win.NativeHandle())
	stage := domain.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: bounds.H}
	if a.hasVideoStageBounds && stage == a.videoStageBounds && dpi == a.videoStageDPI {
		return
	}
	a.videoStageBounds, a.videoStageDPI, a.hasVideoStageBounds = stage, dpi, true
	if a.fullscreen {
		return
	}
	if err := a.platform.LayoutVideo(a.win.NativeHandle(), a.videoHWND, false, stage); err != nil {
		log.Println("video stage layout:", err)
	}
	log.Printf("[layout] MPV video host follows MyGo stage: %+v DIPs", stage)
}

func (a *App) revealVideoHost() {
	if a == nil || a.platform == nil || a.videoHWND == 0 {
		return
	}
	a.layoutVideo()
	if err := a.platform.RevealChild(a.videoHWND); err != nil {
		log.Println("video host:", err)
	}
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Invalidate()
	}
}

func (a *App) hideVideoHost() {
	if a == nil || a.platform == nil || a.videoHWND == 0 {
		return
	}
	a.platform.ShowWindow(a.videoHWND, 0) // SW_HIDE
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Invalidate()
	}
}
