package presentation

import "log"

// setFullscreenCursorHidden balances exactly the ShowCursor counter changes
// made on MyGo's UI thread so the pointer is restored cleanly on exit.
func (a *App) setFullscreenCursorHidden(hidden bool) {
	if a == nil || a.platform == nil {
		return
	}
	if a.cursorHidden == hidden {
		return
	}
	if hidden {
		const maxShowCursorAdjustments = 64
		adjustments := 0
		var displayCount int32
		for adjustments < maxShowCursorAdjustments {
			displayCount = a.platform.ShowCursor(0)
			adjustments++
			if displayCount < 0 {
				break
			}
		}
		if displayCount >= 0 {
			for i := 0; i < adjustments; i++ {
				displayCount = a.platform.ShowCursor(1)
			}
			log.Printf("[cursor] could not hide pointer: display counter remained %d after %d attempts", displayCount, adjustments)
			return
		}
		a.cursorShowAdjustments, a.cursorHidden = adjustments, true
		log.Printf("[cursor] hidden (displayCount=%d, adjustments=%d)", displayCount, adjustments)
		return
	}
	var displayCount int32
	for i := 0; i < a.cursorShowAdjustments; i++ {
		displayCount = a.platform.ShowCursor(1)
	}
	log.Printf("[cursor] restored (displayCount=%d, adjustments=%d)", displayCount, a.cursorShowAdjustments)
	a.cursorShowAdjustments, a.cursorHidden = 0, false
}
