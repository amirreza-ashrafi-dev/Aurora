package application

import "testing"

func TestSeekTarget(t *testing.T) {
	tests := []struct {
		name                            string
		position, delta, duration, want float64
	}{
		{"clamps at zero", 2, -5, 100, 0},
		{"clamps at duration", 95, 10, 100, 100},
		{"keeps relative target", 20, 5, 100, 25},
		{"unknown duration has no upper clamp", 20, 500, 0, 520},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SeekTarget(tt.position, tt.delta, tt.duration); got != tt.want {
				t.Fatalf("SeekTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClampVolume(t *testing.T) {
	for _, tt := range []struct{ input, want float64 }{{-1, 0}, {0, 0}, {37.5, 37.5}, {100, 100}, {120, 100}} {
		if got := ClampVolume(tt.input); got != tt.want {
			t.Errorf("ClampVolume(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
