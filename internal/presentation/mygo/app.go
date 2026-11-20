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
	win          *mygo.Window
	controller   *mygo.Window
	resumeBanner          *mygo.Window
	resumeBannerAnchor    mygo.Rectangle
	fullscreenHeader     *mygo.Window
	fullscreenHeaderAnchor mygo.Rectangle

	platform      application.NativePlatform
	videoHWND     uintptr
	player        application.Player
	playerFactory application.PlayerFactory
	subtitle application.SubtitlePlayer
	subtitlePreferences application.SubtitlePreferencesRepository
	subtitleAssignments application.SubtitleAssignmentRepository
	subtitleAssignmentRecords map[string]domain.SubtitleAssignment
	subtitleCandidates []string
	subtitlePrefs domain.SubtitlePreferences
	subtitleStyle domain.SubtitleStyle
	subtitleDelay float64
	subtitleDisabled bool
	queueThumbnails map[string]*ui.Bitmap
	thumbnailCaptured map[string]bool
	subtitleMenuOpen bool
	fullscreenSubtitleMenuOpen bool
	subtitleFlyout *mygo.Window
	subtitleButtonHandle ui.Handle
	subtitleFlyoutAnchor mygo.Rectangle

	fullscreenSubtitleFlyout *mygo.Window
	fullscreenSubtitleButtonHandle ui.Handle
	fullscreenSubtitleFlyoutAnchor mygo.Rectangle
	fullscreenSubtitleButtonBounds mygo.Rectangle
	colorPickerOpen bool
	activeSubtitleLanguage string
	history        application.ProgressRepository
	historyRecords map[string]domain.PlaybackProgress
	pendingResume  *domain.PlaybackProgress

	progressRequests chan progressWriteRequest
	progressStop     chan struct{}
	progressDone     chan struct{}

	videoStageHandle        ui.Handle
	videoStageBounds        domain.Rect
	videoStageDPI           uint32
	hasVideoStageBounds     bool
	videoLayoutFramePending bool
	videoHeight             int

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
func New(platform application.NativePlatform, playerFactory application.PlayerFactory, history application.ProgressRepository, subtitlePreferences application.SubtitlePreferencesRepository, subtitleAssignments application.SubtitleAssignmentRepository) *App {
	return &App{
		platform:       platform,
		playerFactory:  playerFactory,
		history:        history,
		subtitlePreferences: subtitlePreferences,
		subtitleAssignments: subtitleAssignments,
		subtitleAssignmentRecords: make(map[string]domain.SubtitleAssignment),
		queueThumbnails: make(map[string]*ui.Bitmap),
		thumbnailCaptured: make(map[string]bool),
		subtitlePrefs: domain.SubtitlePreferences{Global: domain.DefaultSubtitleStyle()},
		subtitleStyle: domain.DefaultSubtitleStyle(),
		historyRecords: make(map[string]domain.PlaybackProgress),
		status:         "Starting Aurora Player...",
		state:          domain.PlaybackState{Volume: 100},
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
		Width:             1280,
		Height:            1024,
		MinWidth:          960,
		MinHeight:         720,
		TitleBarStyle:     mygo.TitleBarHidden,
		TitleBarHeight:    40,
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
	if err := a.platform.InstallVideoFileDrop(a.videoHWND, func(paths []string) {
		if a.win == nil || a.win.IsDestroyed() { return }
		a.win.Update(func() {
			if !a.win.IsDestroyed() { a.handleDroppedSubtitleFiles(paths) }
		})
	}); err != nil {
		log.Printf("install video subtitle drop handler: %v", err)
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
		a.loadProgressHistory()
		a.loadSubtitleAssignments()
		a.win.OnFileDrop(func(event *mygo.FileDropEvent) {
			if event == nil { return }
			a.handleDroppedSubtitleFiles(event.Paths)
		})
		a.subtitle, _ = a.player.(application.SubtitlePlayer)
		if a.subtitlePreferences != nil {
			prefs, prefErr := a.subtitlePreferences.Load()
			if prefErr != nil {
				log.Printf("load subtitle preferences: %v", prefErr)
			} else {
				a.subtitlePrefs = prefs
				a.subtitleStyle = application.SubtitleStyleForLanguage(prefs, "und")
				if a.subtitle != nil {
					if styleErr := a.subtitle.SetSubtitleStyle(a.subtitleStyle); styleErr != nil {
						log.Printf("apply default subtitle style: %v", styleErr)
					}
				}
			}
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
		a.closeSubtitleFlyout()
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
