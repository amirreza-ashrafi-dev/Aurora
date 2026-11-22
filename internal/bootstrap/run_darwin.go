//go:build darwin

package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	mygo "github.com/egoist/mygo"

	"my-go/internal/adapters/ffmpeg"
	"my-go/internal/adapters/history"
	"my-go/internal/adapters/mpv"
	"my-go/internal/adapters/subtitles"
	"my-go/internal/application"
	macosplatform "my-go/internal/platform/macos"
	"my-go/internal/platform/macos/videohost"
	mygopresentation "my-go/internal/presentation/mygo"
)

func Run() error {
	platform, err := macosplatform.New()
	if err != nil { return fmt.Errorf("initialize macOS platform: %w", err) }
	playerFactory := func(videoNSView uintptr) (application.Player, error) {
		resourcesDir, err := mygo.App.Path(mygo.PathResources)
		if err != nil { return nil, err }
		return mpv.New(videoNSView, resourcesDir)
	}
	videoHostFactory := func() (application.VideoHost, error) {
		return videohost.New()
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		platform.Close()
		return fmt.Errorf("resolve user configuration directory: %w", err)
	}
	progressRepository, err := history.NewJSONRepository(filepath.Join(configDir, "Aurora Player", "playback_history.json"))
	if err != nil {
		platform.Close()
		return fmt.Errorf("initialize playback history: %w", err)
	}
	subtitleRepository, err := subtitles.NewJSONRepository(filepath.Join(configDir, "Aurora Player"))
	if err != nil {
		platform.Close()
		return fmt.Errorf("initialize subtitle preferences: %w", err)
	}
	resourcesDir, _ := mygo.App.Path(mygo.PathResources)
	filmstripGenerator := ffmpeg.NewFilmstripGenerator(resourcesDir)
	app := mygopresentation.NewWithVideoHost(platform, playerFactory, progressRepository, subtitleRepository, subtitleRepository, videoHostFactory, filmstripGenerator)
	defer app.Close()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Aurora Player: %w", err)
	}
	return nil
}
