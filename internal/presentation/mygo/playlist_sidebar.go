package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/application"
)

func (a *App) playlistSidebarButton(c *ui.Context) {
	tooltip := "Show playlist"
	if a.playlistSidebarOpen {
		tooltip = "Hide playlist"
	}
	if auroraQuietButton(c, "☷").Tooltip(tooltip).Clicked() {
		a.togglePlaylistSidebar()
	}
}

func (a *App) isPlaylistOverlayMode() bool {
	return a != nil && (a.cinematicMode || (a.win != nil && !a.win.IsDestroyed() && a.win.IsFullScreen()))
}

func (a *App) syncPlaylistSidebarPresentation() {
	if a == nil { return }
	if !a.playlistSidebarOpen || !a.isPlaylistOverlayMode() {
		a.hidePlaylistSidebarFlyout()
		return
	}
	a.showPlaylistSidebarFlyout()
}

func (a *App) playlistSidebarFlyoutTarget() (mygo.Rectangle, int, bool) {
	if a == nil || a.win == nil || a.win.IsDestroyed() { return mygo.Rectangle{}, 0, false }
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 || bounds.Height < 1 { return mygo.Rectangle{}, 0, false }
	return mygo.Rectangle{X: bounds.Width, Y: bounds.Height / 2, Width: 1, Height: 1}, bounds.Height, true
}

func (a *App) showPlaylistSidebarFlyout() {
	anchor, height, ok := a.playlistSidebarFlyoutTarget()
	if !ok { return }
	if a.playlistFlyout == nil || a.playlistFlyout.IsDestroyed() {
		a.playlistFlyout = mygo.NewFlyout(mygo.FlyoutOptions{
			Parent: a.win, Anchor: anchor, Placement: mygo.PlacementLeft,
			Gap: 0, Width: auroraQueueWidth + 24, Height: height,
			Focusable: false, Hidden: true,
			Content: ui.View(a.playlistSidebarFlyoutView),
		})
		a.playlistFlyoutAnchor = anchor
	} else {
		// The anchor is window-local, so its rectangle can be unchanged when
		// the player moves to another monitor. SetAnchor deliberately runs on
		// every show to make MyGo resolve the flyout against the parent's
		// current display and work area.
		a.playlistFlyout.SetAnchor(anchor)
		a.playlistFlyoutAnchor = anchor
		a.playlistFlyout.SetSize(auroraQueueWidth+24, height)
	}
	if !a.playlistFlyout.IsVisible() { a.playlistFlyout.ShowInactive() }
	a.playlistFlyout.Invalidate()
}

func (a *App) hidePlaylistSidebarFlyout() {
	if a.playlistFlyout != nil && !a.playlistFlyout.IsDestroyed() && a.playlistFlyout.IsVisible() {
		a.playlistFlyout.Hide()
	}
}

func (a *App) playlistSidebarFlyoutView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	ui.Column(c).Fill().Padding(16).Gap(12).
		Background(ui.Hex(auroraPanel)).Border(1, ui.Hex(auroraBorderSoft)).
		Radius(16).Clip().Children(func() {
			ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "PLAY QUEUE").FontSize(11).Bold().LetterSpacing(1.2).TextColor(ui.Hex(auroraText))
					ui.Text(c, "UP NEXT").FontSize(8).LetterSpacing(1.5).TextColor(ui.Hex(auroraMuted))
				})
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("%02d", len(a.playlist))).FontSize(10).Bold().Padding(6, 9).
					Radius(8).Background(ui.Hex(auroraAccentSoft)).TextColor(ui.Hex(auroraAccent))
				if auroraQuietButton(c, "×").Tooltip("Hide playlist").Clicked() { a.togglePlaylistSidebar() }
			})
			if len(a.playlist) == 0 {
				ui.Column(c).Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).
					Padding(14).Gap(7).Background(ui.Hex("#181611")).
					Border(1, ui.Hex(auroraBorderSoft)).Radius(11).Children(func() {
						ui.Text(c, "Your queue is empty").FontSize(13).Bold().TextColor(ui.Hex(auroraText))
						ui.Text(c, "Add videos here or drop them onto the player.").FontSize(10).TextColor(ui.Hex(auroraMuted))
					})
			} else {
				ui.List(c, nil, len(a.playlist), func(i int) {
					index := i
					mediaPath := a.playlist[i]
					title := queueCardTitle(mediaPath)
					active := index == a.playlistPos
					border := auroraBorderSoft
					if active { border = "#80672D" }
					ui.Row(c).FillWidth().Height(58).Gap(6).Padding(7).AlignItems(ui.Center).
						Background(ui.Hex("#171510")).Border(1, ui.Hex(border)).Radius(9).Children(func() {
							play := ui.Button(c, "").Label("Play "+title).Grow(1).Shrink(1).MinWidth(0).Height(44).
								Padding(0).Background(ui.Transparent).Border(0, ui.Transparent)
							play.Children(func() {
								ui.Column(c).Fill().Grow(1).Shrink(1).MinWidth(0).Gap(4).
									AlignItems(ui.Start).Children(func() {
										color := auroraText
										if active { color = "#F2D995" }
										ui.Text(c, title).FillWidth().MinWidth(0).SingleLine().FontSize(10).Bold().
											TextColor(ui.Hex(color)).Tooltip(title)
										meta := strings.ToUpper(strings.TrimPrefix(filepath.Ext(mediaPath), "."))
										if meta == "" { meta = "VIDEO" }
										if active { meta = "PLAYING · " + meta }
										ui.Text(c, meta).FontSize(8).TextColor(ui.Hex(auroraMuted))
									})
							})
							if play.Clicked() { a.playPlaylistIndex(index) }
							if auroraQuietButton(c, "×").Tooltip("Remove from queue").Clicked() {
								a.queuePlaylistRemoval(mediaPath)
								c.After(time.Millisecond)
							}
						})
				}).Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).Gap(7)
			}
			ui.Column(c).Shrink(0).Gap(8).Children(func() {
				if auroraAddVideosButton(c).Clicked() { a.chooseFiles() }
				ui.Text(c, "Drop another video onto the player to add and play it").FontSize(8).TextColor(ui.Hex(auroraMuted))
			})
		})
}

func (a *App) handleDroppedMediaFiles(paths []string) {
	videoPaths := application.FilterVideoLaunchPaths(paths)
	subtitlePaths := make([]string, 0, len(paths))
	for _, path := range paths {
		if application.IsSubtitleFile(path) { subtitlePaths = append(subtitlePaths, path) }
	}
	if len(videoPaths) > 0 { a.addDroppedVideosAndPlayLatest(videoPaths) }
	if len(subtitlePaths) > 0 { a.handleDroppedSubtitleFiles(subtitlePaths) }
	if a.win != nil && !a.win.IsDestroyed() { a.win.Invalidate() }
	if a.playlistFlyout != nil && !a.playlistFlyout.IsDestroyed() { a.playlistFlyout.Invalidate() }
}

func (a *App) addDroppedVideosAndPlayLatest(paths []string) {
	if a == nil || len(paths) == 0 { return }
	previousCount := len(a.playlist)
	a.addFiles(paths)
	if len(a.playlist) <= previousCount { return }
	latestIndex := len(a.playlist) - 1
	a.playPlaylistIndex(latestIndex)
	a.status = "Playing dropped video: " + filepath.Base(a.path)
}
