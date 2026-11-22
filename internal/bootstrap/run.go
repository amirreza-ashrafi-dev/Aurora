//go:build !darwin

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
	winplatform "my-go/internal/platform/windows"
	mygopresentation "my-go/internal/presentation/mygo"
)

// Run is the composition root: it creates concrete adapters and injects them
// into the MyGo presentation controller through application-owned ports.
func Run() error {
	platform, err := winplatform.New()
	if err != nil {
		return fmt.Errorf("initialize Windows platform: %w", err)
	}
	playerFactory := func(videoHWND uintptr) (application.Player, error) {
		resourcesDir, err := mygo.App.Path(mygo.PathResources)
		if err != nil {
			return nil, err
		}
		return mpv.New(videoHWND, resourcesDir)
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
	app := mygopresentation.New(platform, playerFactory, progressRepository, subtitleRepository, subtitleRepository, filmstripGenerator)
	defer app.Close()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Aurora Player: %w", err)
	}
	return nil
}
