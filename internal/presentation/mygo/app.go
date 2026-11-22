package presentation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/application"
	"my-go/internal/domain"
	"my-go/resources"
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
	volumeOSD *mygo.Window
	volumeOSDUntil time.Time
	wheelRemainder int32

	memoryWindow *mygo.Window
	memoryMonitorStop chan struct{}
	memorySamples []memorySample
	memoryExportPath string
	memoryGCInProgress bool
	thumbnailRasterBytes map[string]uint64
	thumbnailEncodedBytes map[string]uint64
	thumbnailCaptureTokens map[string]uint64
	thumbnailCaptureSequence uint64

	filmstripGenerator application.FilmstripGenerator
	filmstripOpen bool
	filmstripManifest domain.Filmstrip
	filmstripManifestPath string
	filmstripGenerating bool
	filmstripProgressCompleted int
	filmstripProgressTotal int
	filmstripGeneratingFor string
	filmstripGenerationAttemptedFor string
	filmstripError string
	filmstripGenerationCancel context.CancelFunc
	filmstripGenerationToken uint64
	filmstripPreviewPosition float64
	filmstripDragging bool
	filmstripHoverPinned bool
	filmstripHoverStart int
	filmstripSelectedFrame int
	filmstripHasSelectedFrame bool
	filmstripWheelRemainder float32
	filmstripBitmaps map[string]*filmstripBitmap
	filmstripBitmapClock uint64
	filmstripLoadInFlight bool
	filmstripLoadToken uint64
	timelinePreviewPosition float64

	platform      application.NativePlatform
	videoHWND     uintptr
	player        application.Player
	playerFactory application.PlayerFactory
	// Serializes background MPV thumbnail commands against player teardown.
	playerCommandMu sync.RWMutex
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

	playlistFlyout *mygo.Window
	playlistFlyoutAnchor mygo.Rectangle
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
	startupFiles []string
	startupFilesTimer *time.Timer
	windowReady bool
	playlist    []string
	playlistPos int
	playlistSidebarOpen bool
	pendingPlaylistRemoval []string
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
func New(platform application.NativePlatform, playerFactory application.PlayerFactory, history application.ProgressRepository, subtitlePreferences application.SubtitlePreferencesRepository, subtitleAssignments application.SubtitleAssignmentRepository, filmstripGenerators ...application.FilmstripGenerator) *App {
	app := &App{
		platform:       platform,
		playerFactory:  playerFactory,
		history:        history,
		subtitlePreferences: subtitlePreferences,
		subtitleAssignments: subtitleAssignments,
		subtitleAssignmentRecords: make(map[string]domain.SubtitleAssignment),
		queueThumbnails: make(map[string]*ui.Bitmap),
		thumbnailCaptured: make(map[string]bool),
		thumbnailRasterBytes: make(map[string]uint64),
		thumbnailEncodedBytes: make(map[string]uint64),
		thumbnailCaptureTokens: make(map[string]uint64),
		subtitlePrefs: domain.SubtitlePreferences{Global: domain.DefaultSubtitleStyle()},
		subtitleStyle: domain.DefaultSubtitleStyle(),
		historyRecords: make(map[string]domain.PlaybackProgress),
		status:         "Starting Aurora Player...",
		state:          domain.PlaybackState{Volume: 100},
		playlistSidebarOpen: false,
	}
	if len(filmstripGenerators) > 0 {
		app.filmstripGenerator = filmstripGenerators[0]
	}
	return app
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

	// Set the native window icon explicitly so development runs (go run/go tool
	// mygo dev) show Aurora's logo in the Windows taskbar as well as packaged
	// builds, which take their executable icon from mygo.json.
	if err := a.win.SetIcon(resources.IconPNG); err != nil {
		log.Printf("set Aurora window icon: %v", err)
	}

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
			if !a.win.IsDestroyed() { a.handleDroppedMediaFiles(paths) }
		})
	}); err != nil {
		log.Printf("install video subtitle drop handler: %v", err)
	}
	if err := a.platform.InstallVideoMouseWheel(a.videoHWND, func(delta int32) {
		if a.win == nil || a.win.IsDestroyed() { return }
		a.win.Update(func() {
			if !a.win.IsDestroyed() { a.handleVolumeWheel(delta) }
		})
	}); err != nil {
		log.Printf("install video mouse-wheel handler: %v", err)
	}

	// Keep libmpv unloaded until the first valid video is added. The native
	// video host already exists, so the player can attach to it on demand.
	a.status = "Ready — add videos to your playlist"
	a.loadProgressHistory()
	a.loadSubtitleAssignments()
	if a.subtitlePreferences != nil {
		prefs, prefErr := a.subtitlePreferences.Load()
		if prefErr != nil {
			log.Printf("load subtitle preferences: %v", prefErr)
		} else {
			a.subtitlePrefs = prefs
			a.subtitleStyle = application.SubtitleStyleForLanguage(prefs, "und")
		}
	}
	a.win.OnFileDrop(func(event *mygo.FileDropEvent) {
		if event == nil { return }
		a.handleDroppedSubtitleFiles(event.Paths)
	})

	a.win.OnReadyToShow(func() {
		a.createFullscreenController()
		a.startControllerMonitor()
		a.layoutVideo()
		a.updateControllerBounds()
		a.logDiagnostics("ready-to-show")
		a.windowReady = true
		a.openQueuedFiles()
	})
	a.win.OnResize(func() {
		a.videoLayoutFramePending = false
		a.win.Invalidate()
		a.updateControllerBounds()
		a.syncPlaylistSidebarPresentation()
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
		a.syncPlaylistSidebarPresentation()
		a.setFullscreenCursorHidden(false)
		a.layoutVideo()
		a.updateControllerBounds()
		a.showController()
		a.logDiagnostics("enter-fullscreen")
	})
	a.win.OnLeaveFullScreen(func() {
		a.fullscreen = false
		a.syncPlaylistSidebarPresentation()
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


// ensurePlayer initializes libmpv only when a valid video is about to be
// loaded. This keeps the empty player free of MPV's decoder and render stack.
func (a *App) ensurePlayer() error {
	if a == nil {
		return fmt.Errorf("player is unavailable")
	}
	if a.player != nil {
		// Keep the initialized context across queue sessions; only polling stops.
		a.startPolling()
		return nil
	}
	if a.playerFactory == nil {
		return fmt.Errorf("MPV player is unavailable on this platform")
	}
	if a.videoHWND == 0 {
		return fmt.Errorf("video surface is not ready")
	}
	player, err := a.playerFactory(a.videoHWND)
	if err != nil {
		return err
	}
	if player == nil {
		return fmt.Errorf("MPV player factory returned no player")
	}
	if err := player.SetVolume(a.state.Volume); err != nil {
		player.Close()
		return fmt.Errorf("set initial player volume: %w", err)
	}
	a.playerCommandMu.Lock()
	a.player = player
	a.subtitle, _ = player.(application.SubtitlePlayer)
	a.playerCommandMu.Unlock()
	if a.subtitle != nil {
		if err := a.subtitle.SetSubtitleStyle(a.subtitleStyle); err != nil {
			log.Printf("apply default subtitle style: %v", err)
		}
	}
	a.startPolling()
	return nil
}

// unloadPlayerMedia stops playback and releases the GPU video output when the
// linked MPV build supports runtime VO reinitialization. Older MPV builds
// cannot safely reconfigure the VO at runtime, so release their whole context.
func (a *App) unloadPlayerMedia() error {
	if a == nil {
		return nil
	}
	a.playerCommandMu.Lock()
	defer a.playerCommandMu.Unlock()

	player := a.player
	if player == nil {
		return nil
	}
	if managed, ok := player.(interface {
		SupportsRuntimeVideoOutputRelease() bool
		Unload() error
	}); ok {
		if !managed.SupportsRuntimeVideoOutputRelease() {
			log.Printf("MPV build does not support runtime video-output release; destroying its context")
			a.closePlayerLocked()
			return nil
		}
		return managed.Unload()
	}
	if unloader, ok := player.(interface{ Unload() error }); ok {
		return unloader.Unload()
	}
	return player.Stop()
}

// closePlayer waits for any in-flight background MPV command before destroying
// the context or unloading its DLL. Callers must stop the polling goroutine first.
func (a *App) closePlayer() {
	a.playerCommandMu.Lock()
	defer a.playerCommandMu.Unlock()
	a.closePlayerLocked()
}

// closePlayerLocked releases the player while playerCommandMu is held.
func (a *App) closePlayerLocked() {
	player := a.player
	a.player = nil
	a.subtitle = nil
	if player != nil {
		player.Close()
	}
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
