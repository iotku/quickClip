package main

import "github.com/gopxl/beep/v2"

// TapStreamer wraps another streamer and captures its samples.
type TapStreamer struct {
	s beep.Streamer // underlying streamer
}

// Err just passes through the error from the wrapped streamer.
func (t *TapStreamer) Err() error {
	return t.s.Err()
}

func (t *TapStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = t.s.Stream(samples)
	// If no samples were produced or the streamer signals it's finished, don't update.
	if n == 0 || !ok {
		return n, ok
	}
	// Convert the float64 samples (in [-1,1]) to a PCM byte slice.
	buf := make([]byte, n*4) // 4 bytes per sample (2 channels x 2 bytes)
	for i := 0; i < n; i++ {
		left := int16(samples[i][0] * 32767)
		right := int16(samples[i][1] * 32767)
		buf[i*4+0] = byte(left)
		buf[i*4+1] = byte(left >> 8)
		buf[i*4+2] = byte(right)
		buf[i*4+3] = byte(right >> 8)
	}
	// Optionally, check if the buffer is silent (all zeros)
	if isSilence(buf) {
		return n, ok
	}
	// Only update visualization with valid, non-silent data.
	updateVisualization(buf)
	return n, ok
}

// Helper function to check if the buffer is silent.
func isSilence(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}
