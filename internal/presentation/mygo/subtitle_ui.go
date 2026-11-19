package presentation

import (
	"fmt"
	"path/filepath"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/application"
	"my-go/internal/domain"
)

func (a *App) chooseSubtitleFile() {
	if a.win == nil || a.subtitle == nil {
		a.status = "Subtitle playback is unavailable"
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title: "Load subtitle file",
			Multiple: false,
			Filters: []mygo.FileFilter{
				{Name: "Subtitle files", Extensions: []string{"srt", "ass", "ssa", "vtt", "sub"}},
				{Name: "All files", Extensions: []string{"*"}},
			},
		})
		if err != nil {
			if a.win != nil && !a.win.IsDestroyed() {
				a.win.Update(func() { a.status = "Subtitle picker: " + err.Error() })
			}
			return
		}
		if len(paths) == 0 || a.win == nil || a.win.IsDestroyed() { return }
		a.win.Update(func() {
			if err := a.subtitle.LoadExternalSubtitle(paths[0]); err != nil {
				a.status = "Load subtitle: " + err.Error()
				return
			}
			a.status = "Subtitle loaded: " + filepath.Base(paths[0])
			a.subtitleMenuOpen = false
			a.win.Invalidate()
		})
	}()
}

func (a *App) applySubtitleStyle(style domain.SubtitleStyle) {
	if err := application.ValidateSubtitleStyle(style); err != nil {
		a.status = err.Error()
		return
	}
	if a.subtitle == nil {
		a.status = "Subtitle playback is unavailable"
		return
	}
	if err := a.subtitle.SetSubtitleStyle(style); err != nil {
		a.status = "Subtitle style: " + err.Error()
		return
	}
	a.subtitleStyle = style
	a.subtitlePrefs.Global = style
	a.subtitlePrefs.Version = 1
	if a.subtitlePreferences != nil {
		if err := a.subtitlePreferences.Save(a.subtitlePrefs); err != nil {
			a.status = "Save subtitle preferences: " + err.Error()
			return
		}
	}
	a.status = "Subtitle appearance updated"
}

func (a *App) adjustSubtitleDelay(delta float64) {
	if a.subtitle == nil { return }
	next := a.subtitleDelay + delta
	if next < -30 { next = -30 }
	if next > 30 { next = 30 }
	if err := a.subtitle.SetSubtitleDelay(next); err != nil {
		a.status = err.Error()
		return
	}
	a.subtitleDelay = next
	a.status = fmt.Sprintf("Subtitle delay: %+.1fs", next)
	if a.win != nil && !a.win.IsDestroyed() { a.win.Invalidate() }
}

func (a *App) resetSubtitleDelay() {
	if a.subtitle == nil { return }
	if err := a.subtitle.SetSubtitleDelay(0); err != nil { a.status = err.Error(); return }
	a.subtitleDelay = 0
	a.status = "Subtitle delay reset"
	if a.win != nil && !a.win.IsDestroyed() { a.win.Invalidate() }
}

func (a *App) subtitleStudioControls(c *ui.Context) {
	ui.Column(c).FillWidth().Padding(10).Gap(7).Background(ui.Hex("#171D28")).Border(1, ui.Hex(auroraBorder)).Radius(11).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Text(c, "SUBTITLE STUDIO").FontSize(10).Bold().LetterSpacing(1).TextColor(ui.Hex(auroraText))
			ui.Spacer(c)
			if auroraQuietButton(c, "Load file…").Clicked() { a.chooseSubtitleFile() }
			if auroraQuietButton(c, "Disable").Clicked() {
				if a.subtitle != nil {
					if err := a.subtitle.DisableSubtitles(); err != nil { a.status = err.Error() } else { a.status = "Subtitles disabled" }
				}
			}
		})
		ui.Row(c).Gap(7).Children(func() {
			ui.Text(c, "FONT").FontSize(8).TextColor(ui.Hex(auroraMuted))
			for _, font := range []string{"Segoe UI", "Arial", "Tahoma", "Yu Gothic"} {
				if auroraQuietButton(c, font).Clicked() {
					style := a.subtitleStyle; style.FontFamily = font; a.applySubtitleStyle(style)
				}
			}
			ui.Spacer(c)
			if auroraQuietButton(c, "A−").Clicked() { style:=a.subtitleStyle; style.FontSize-=2; a.applySubtitleStyle(style) }
			ui.Text(c, fmt.Sprintf("%.0f px", a.subtitleStyle.FontSize)).FontSize(9).TextColor(ui.Hex(auroraText))
			if auroraQuietButton(c, "A+").Clicked() { style:=a.subtitleStyle; style.FontSize+=2; a.applySubtitleStyle(style) }
		})
		ui.Row(c).Gap(7).Children(func() {
			ui.Text(c, "TEXT").FontSize(8).TextColor(ui.Hex(auroraMuted))
			for _, color := range []string{"#FFFFFFFF", "#FFFFD166", "#FFFF6B6B", "#FF8BE9FD"} {
				if auroraQuietButton(c, color).Clicked() { style:=a.subtitleStyle; style.TextColor=color; a.applySubtitleStyle(style) }
			}
			ui.Text(c, "OUTLINE").FontSize(8).TextColor(ui.Hex(auroraMuted))
			if auroraQuietButton(c, "−").Clicked() { style:=a.subtitleStyle; style.OutlineSize-=0.5; a.applySubtitleStyle(style) }
			ui.Text(c, fmt.Sprintf("%.1f", a.subtitleStyle.OutlineSize)).FontSize(9).TextColor(ui.Hex(auroraText))
			if auroraQuietButton(c, "+").Clicked() { style:=a.subtitleStyle; style.OutlineSize+=0.5; a.applySubtitleStyle(style) }
		})
		ui.Row(c).Gap(7).Children(func() {
			ui.Text(c, "POSITION").FontSize(8).TextColor(ui.Hex(auroraMuted))
			if auroraQuietButton(c, "Up").Clicked() { style:=a.subtitleStyle; style.VerticalPosition-=2; a.applySubtitleStyle(style) }
			ui.Text(c, fmt.Sprintf("%d", a.subtitleStyle.VerticalPosition)).FontSize(9).TextColor(ui.Hex(auroraText))
			if auroraQuietButton(c, "Down").Clicked() { style:=a.subtitleStyle; style.VerticalPosition+=2; a.applySubtitleStyle(style) }
			ui.Spacer(c)
			ui.Text(c, "DELAY").FontSize(8).TextColor(ui.Hex(auroraMuted))
			if auroraQuietButton(c, "−0.1s").Clicked() { a.adjustSubtitleDelay(-0.1) }
			ui.Text(c, fmt.Sprintf("%+.1fs", a.subtitleDelay)).FontSize(9).TextColor(ui.Hex(auroraText))
			if auroraQuietButton(c, "+0.1s").Clicked() { a.adjustSubtitleDelay(0.1) }
			if auroraQuietButton(c, "Reset").Clicked() { a.resetSubtitleDelay() }
		})
		ui.Text(c, "ASS/SSA and image-based subtitles may not support every appearance setting.").FontSize(8).TextColor(ui.Hex(auroraMuted))
	})
}

