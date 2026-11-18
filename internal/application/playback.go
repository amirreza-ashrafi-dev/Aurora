package application

// SeekTarget clamps a relative seek to the known media interval.
func SeekTarget(position, delta, duration float64) float64 {
	target := position + delta
	if target < 0 {
		target = 0
	}
	if duration > 0 && target > duration {
		target = duration
	}
	return target
}

// ClampVolume constrains volume to the supported 0-100 range.
func ClampVolume(volume float64) float64 {
	if volume < 0 {
		return 0
	}
	if volume > 100 {
		return 100
	}
	return volume
}
