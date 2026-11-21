package presentation

import (
	"testing"

	"my-go/internal/domain"
)

type timelineTestPlayer struct {
	seekCalls []float64
}

func (*timelineTestPlayer) Load(string) error                 { return nil }
func (*timelineTestPlayer) Append(string) error               { return nil }
func (*timelineTestPlayer) PlayIndex(int) error               { return nil }
func (*timelineTestPlayer) RemovePlaylistItem(int) error      { return nil }
func (*timelineTestPlayer) GetDouble(string) (float64, error) { return 0, nil }
func (*timelineTestPlayer) SetPaused(bool) error              { return nil }
func (p *timelineTestPlayer) Seek(position float64) error {
	p.seekCalls = append(p.seekCalls, position)
	return nil
}
func (*timelineTestPlayer) SetVolume(float64) error { return nil }
func (*timelineTestPlayer) Stop() error             { return nil }
func (*timelineTestPlayer) Close()                  {}

func TestTimelineDraggingSeeksOnlyOnceOnRelease(t *testing.T) {
	player := &timelineTestPlayer{}
	app := &App{
		player: player,
		path:   "movie.mkv",
		state:  domain.PlaybackState{Position: 5, Duration: 120},
	}

	app.timelinePreviewPosition = 12
	app.updateTimeline(true, true)
	app.timelinePreviewPosition = 18
	app.updateTimeline(true, true)
	app.timelinePreviewPosition = 24
	app.updateTimeline(true, true)

	if len(player.seekCalls) != 0 {
		t.Fatalf("seeks while dragging = %d, want 0", len(player.seekCalls))
	}

	app.updateTimeline(false, false)
	if len(player.seekCalls) != 1 {
		t.Fatalf("seek calls after release = %d, want 1", len(player.seekCalls))
	}
	if player.seekCalls[0] != 24 {
		t.Fatalf("seek position = %v, want 24", player.seekCalls[0])
	}
	if app.state.Position != 24 {
		t.Fatalf("committed position = %v, want 24", app.state.Position)
	}
}

func TestTimelineClickCommitsOneSeek(t *testing.T) {
	player := &timelineTestPlayer{}
	app := &App{
		player: player,
		path:   "movie.mkv",
		state:  domain.PlaybackState{Position: 5, Duration: 120},
		timelinePreviewPosition: 37,
	}
	app.updateTimeline(false, true)
	if len(player.seekCalls) != 1 || player.seekCalls[0] != 37 {
		t.Fatalf("seek calls = %#v, want one seek to 37", player.seekCalls)
	}
}

func TestTimelineCancellationRestoresCommittedPosition(t *testing.T) {
	player := &timelineTestPlayer{}
	app := &App{
		player: player,
		path:   "movie.mkv",
		state:  domain.PlaybackState{Position: 5, Duration: 120},
		scrubbing: true,
		timelinePreviewPosition: 37,
	}
	app.updateTimeline(false, false, true)
	if len(player.seekCalls) != 0 {
		t.Fatalf("canceled timeline drag issued %d seeks, want 0", len(player.seekCalls))
	}
	if app.timelinePreviewPosition != 5 {
		t.Fatalf("preview position after cancellation = %v, want 5", app.timelinePreviewPosition)
	}
}
