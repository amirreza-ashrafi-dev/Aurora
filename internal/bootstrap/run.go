package bootstrap

import (
	"fmt"

	mygo "github.com/egoist/mygo"

	"my-go/internal/adapters/mpv"
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
	app := mygopresentation.New(platform, playerFactory)
	defer app.Close()
	if err := app.Run(); err != nil {
		return fmt.Errorf("run Aurora Player: %w", err)
	}
	return nil
}
