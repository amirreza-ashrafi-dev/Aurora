package presentation

import (
	"fmt"
	"os"
	"strings"
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
			a.persistSubtitleAssignment(paths[0], application.SubtitleLanguageFromFilename(a.path, paths[0]), false)
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
		if err := a.subtitlePreferences.SavePreferences(a.subtitlePrefs); err != nil {
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
					if err := a.subtitle.DisableSubtitles(); err != nil { a.status = err.Error() } else {
						a.persistSubtitleAssignment("", "", true)
						a.status = "Subtitles disabled"
					}
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
		ui.Row(c).Gap(7).Children(func() {
			if auroraQuietButton(c, "Next track").Clicked() { if tracks, ok := a.subtitle.(interface{ CycleSubtitleTrack() error }); ok { if err := tracks.CycleSubtitleTrack(); err != nil { a.status = err.Error() } else { a.status = "Subtitle track changed" } } }
			if auroraQuietButton(c, "Remove external").Clicked() { if tracks, ok := a.subtitle.(interface{ RemoveSelectedExternalSubtitle() error }); ok { if err := tracks.RemoveSelectedExternalSubtitle(); err != nil { a.status = err.Error() } else { a.persistSubtitleAssignment("", "", true); a.status = "External subtitle removed" } } }
			if auroraQuietButton(c, "Save for language").Clicked() { a.saveStyleForActiveLanguage() }
		})
		if len(a.subtitleCandidates) > 0 {
			ui.Text(c, "MATCHING FILES").FontSize(8).Bold().TextColor(ui.Hex(auroraMuted))
			for _, candidate := range a.subtitleCandidates {
				path := candidate
				if auroraQuietButton(c, filepath.Base(path)).Clicked() {
					if err := a.subtitle.LoadExternalSubtitle(path); err != nil { a.status = err.Error() } else {
						a.persistSubtitleAssignment(path, application.SubtitleLanguageFromFilename(a.path, path), false)
						a.status = "Subtitle selected: " + filepath.Base(path)
						a.subtitleCandidates = nil
					}
				}
			}
		}
		textColor := subtitleColorFromMPV(a.subtitleStyle.TextColor)
		previousTextColor := textColor
		ui.Text(c, "TEXT COLOR").FontSize(8).TextColor(ui.Hex(auroraMuted))
		ui.ColorWell(c, &textColor)
		if textColor != previousTextColor {
			style := a.subtitleStyle
			style.TextColor = fmt.Sprintf("#FF%02X%02X%02X", textColor.R, textColor.G, textColor.B)
			a.applySubtitleStyle(style)
		}
		outlineColor := subtitleColorFromMPV(a.subtitleStyle.OutlineColor)
		previousOutlineColor := outlineColor
		ui.Text(c, "OUTLINE COLOR").FontSize(8).TextColor(ui.Hex(auroraMuted))
		ui.ColorWell(c, &outlineColor)
		if outlineColor != previousOutlineColor {
			style := a.subtitleStyle
			style.OutlineColor = fmt.Sprintf("#FF%02X%02X%02X", outlineColor.R, outlineColor.G, outlineColor.B)
			a.applySubtitleStyle(style)
		}
		ui.Text(c, "ASS/SSA and image-based subtitles may not support every appearance setting.").FontSize(8).TextColor(ui.Hex(auroraMuted))
	})
}


func (a *App) saveStyleForActiveLanguage() {
	language := application.NormalizeSubtitleLanguage(a.activeSubtitleLanguage)
	if language == "und" { a.status = "The active subtitle has no language metadata"; return }
	for i := range a.subtitlePrefs.Languages {
		if application.NormalizeSubtitleLanguage(a.subtitlePrefs.Languages[i].Language) == language {
			a.subtitlePrefs.Languages[i].Style = a.subtitleStyle
			if a.subtitlePreferences != nil { if err := a.subtitlePreferences.SavePreferences(a.subtitlePrefs); err != nil { a.status = err.Error(); return } }
			a.status = "Saved subtitle style for " + language
			return
		}
	}
	a.subtitlePrefs.Languages = append(a.subtitlePrefs.Languages, domain.SubtitleLanguageProfile{Language: language, Style: a.subtitleStyle})
	if a.subtitlePreferences != nil { if err := a.subtitlePreferences.SavePreferences(a.subtitlePrefs); err != nil { a.status = err.Error(); return } }
	a.status = "Saved subtitle style for " + language
}

func (a *App) loadSubtitleAssignments() {
	if a.subtitleAssignments == nil { return }
	records, err := a.subtitleAssignments.LoadAll()
	if err != nil { a.status = "Load subtitle assignments: " + err.Error(); return }
	for _, record := range records { a.subtitleAssignmentRecords[subtitlePathKey(record.MediaPath)] = record }
}

func (a *App) persistSubtitleAssignment(externalPath, language string, disabled bool) {
	if a.path == "" || a.subtitleAssignments == nil { return }
	record := domain.SubtitleAssignment{MediaPath: a.path, ExternalPath: externalPath, Language: language, Disabled: disabled}
	if err := a.subtitleAssignments.SaveAssignment(record); err != nil { a.status = "Save subtitle assignment: " + err.Error(); return }
	a.subtitleAssignmentRecords[subtitlePathKey(a.path)] = record
}

func (a *App) restoreSubtitleForCurrent() {
	if a.subtitle == nil || a.path == "" { return }
	key := subtitlePathKey(a.path)
	record, found := a.subtitleAssignmentRecords[key]
	if !found {
		matches, err := application.DiscoverSubtitleFiles(a.path)
		if err != nil { return }
		a.subtitleCandidates = matches
		if len(matches) > 1 { a.subtitleMenuOpen = true; a.status = fmt.Sprintf("%d matching subtitles found for %s", len(matches), filepath.Base(a.path)) }
		if len(matches) == 1 {
			lang := application.SubtitleLanguageFromFilename(a.path, matches[0])
			if err := a.subtitle.LoadExternalSubtitle(matches[0]); err == nil { a.persistSubtitleAssignment(matches[0], lang, false) }
		}
		return
	}
	if record.Disabled { _ = a.subtitle.DisableSubtitles(); return }
	if record.ExternalPath != "" {
		if _, err := os.Stat(record.ExternalPath); err != nil { a.status = "Saved subtitle is missing: " + filepath.Base(record.ExternalPath); return }
		if err := a.subtitle.LoadExternalSubtitle(record.ExternalPath); err != nil { a.status = "Restore subtitle: " + err.Error(); return }
		a.status = "Restored subtitle: " + filepath.Base(record.ExternalPath)
	}
}

func (a *App) handleDroppedSubtitleFiles(paths []string) {
	if a.subtitle == nil || a.path == "" { return }
	matches := application.MatchSubtitleFiles(a.path, paths)
	if len(matches) == 0 { a.status = "Dropped files do not match the current video"; return }
	a.subtitleCandidates = matches
	if len(matches) == 1 {
		lang := application.SubtitleLanguageFromFilename(a.path, matches[0])
		if err := a.subtitle.LoadExternalSubtitle(matches[0]); err != nil { a.status = err.Error(); return }
		a.persistSubtitleAssignment(matches[0], lang, false)
		a.status = "Loaded dropped subtitle: " + filepath.Base(matches[0])
	} else {
		a.status = fmt.Sprintf("%d matching subtitle files found; choose one in Subtitle Studio", len(matches))
		a.subtitleMenuOpen = true
	}
	if a.win != nil && !a.win.IsDestroyed() { a.win.Invalidate() }
}

func subtitlePathKey(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil { path = abs }
	return strings.ToLower(filepath.Clean(path))
}

func subtitleColorFromMPV(value string) ui.Color {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 8 { value = value[2:] } // discard MPV alpha for the UI swatch
	if len(value) != 6 { return ui.Hex("#FFFFFF") }
	var r, g, b uint8
	if _, err := fmt.Sscanf(value, "%02X%02X%02X", &r, &g, &b); err != nil { return ui.Hex("#FFFFFF") }
	return ui.Color{R: r, G: g, B: b, A: 255}
}
