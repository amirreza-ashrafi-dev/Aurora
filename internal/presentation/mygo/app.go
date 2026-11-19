package presentation

import (
	"fmt"
	"log"
	"sync"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/application"
	"my-go/internal/domain"
)

// App coordinates MyGo windows and presentation state while depending only on
// application ports for MPV playback and native Windows operations.
type App struct {
	win        *mygo.Window
	controller *mygo.Window

	platform      application.NativePlatform
	videoHWND     uintptr
	player        application.Player
	playerFactory application.PlayerFactory

	videoStageHandle        ui.Handle
	videoStageBounds        domain.Rect
	videoStageDPI           uint32
	hasVideoStageBounds     bool
	videoLayoutFramePending bool

	path        string
	playlist    []string
	playlistPos int
	status      string
	state       domain.PlaybackState
	scrubbing   bool
	fullscreen     bool
	cinematicMode bool

	pollStop       chan struct{}
	pollDone       chan struct{}
	controllerStop chan struct{}
	controllerDone chan struct{}

	controllerAnchor    mygo.Rectangle
	hasControllerAnchor bool

	cursorHidden          bool
	cursorShowAdjustments int
	closeOnce             sync.Once
}

// New creates the presentation controller. The player is created later, after
// MyGo has supplied the native window handle for the embedded MPV surface.
func New(platform application.NativePlatform, playerFactory application.PlayerFactory) *App {
	return &App{
		platform:      platform,
		playerFactory: playerFactory,
		status:        "Starting Aurora Player...",
		state:         domain.PlaybackState{Volume: 100},
	}
}

// Run starts MyGo and installs the native player window when the app is ready.
func (a *App) Run() error {
	mygo.App.WhenReady(a.onReady)
	return mygo.App.Run()
}

func (a *App) onReady() {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "Aurora Player",
		Width:             1200,
		Height:            900,
		MinWidth:          950,
		MinHeight:         720,
		TitleBarStyle:     mygo.TitleBarHidden,
		TitleBarHeight:    54,
		HideWindowButtons: true,
		Content:           ui.View(a.view),
	})

	var err error
	a.videoHWND, err = a.platform.CreateVideoHost(a.win.NativeHandle())
	if err != nil {
		a.status = err.Error()
		log.Println(err)
		return
	}
	if err := a.platform.InstallVideoDoubleClick(a.videoHWND, a.toggleFullscreen); err != nil {
		a.status = "Double-click handler: " + err.Error()
		log.Println(a.status)
	}

	if a.playerFactory != nil {
		a.player, err = a.playerFactory(a.videoHWND)
	}
	if err != nil {
		a.status = err.Error()
		log.Println(err)
	} else if a.player == nil {
		a.status = "libmpv is not initialized"
	} else {
		a.status = "Ready — add videos to your playlist"
		if err := a.player.SetVolume(a.state.Volume); err != nil {
			a.status = err.Error()
		}
		a.startPolling()
	}

	a.win.OnReadyToShow(func() {
		a.createFullscreenController()
		a.startControllerMonitor()
		a.layoutVideo()
		a.updateControllerBounds()
		a.logDiagnostics("ready-to-show")
	})
	a.win.OnResize(func() {
		a.videoLayoutFramePending = false
		a.win.Invalidate()
		a.updateControllerBounds()
		a.logDiagnostics("resize")
	})
	a.win.OnMaximize(func() { a.videoLayoutFramePending = false; a.win.Invalidate() })
	a.win.OnUnmaximize(func() { a.videoLayoutFramePending = false; a.win.Invalidate() })
	a.win.OnMove(func() {
		a.win.Invalidate()
		a.updateControllerBounds()
		a.logDiagnostics("move")
	})
	a.win.OnEnterFullScreen(func() {
		a.fullscreen = true
		a.setFullscreenCursorHidden(false)
		a.layoutVideo()
		a.updateControllerBounds()
		a.showController()
		a.logDiagnostics("enter-fullscreen")
	})
	a.win.OnLeaveFullScreen(func() {
		a.fullscreen = false
		if !a.cinematicMode {
			a.setFullscreenCursorHidden(false)
			a.hideController()
		}
		a.videoLayoutFramePending = false
		a.win.Invalidate()
		a.layoutVideo()
		a.updateControllerBounds()
		a.logDiagnostics("leave-fullscreen")
	})
}

func formatTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	hours, minutes, secs := total/3600, (total%3600)/60, total%60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func (a *App) currentTitle() string { return (domain.Media{Path: a.path}).Title() }
