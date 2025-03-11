package main

import "github.com/gopxl/beep/v2"

// TapStreamer wraps another streamer and captures its samples.
type TapStreamer struct {
	s beep.Streamer // underlying streamer
}

// Stream intercepts the samples, sends a copy to the visualization,
// and passes the samples along.
func (t *TapStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = t.s.Stream(samples)
	// Convert the float64 samples (range -1 to +1) to 16-bit PCM bytes.
	// Here we allocate a byte buffer (4 bytes per sample: 2 channels x 2 bytes).
	buf := make([]byte, n*4)
	for i := 0; i < n; i++ {
		// Convert each float64 sample to an int16. We assume the sample is in [-1, 1].
		// Multiply by 32767 to scale to the int16 range.
		left := int16(samples[i][0] * 32767)
		right := int16(samples[i][1] * 32767)
		// Write the 16-bit values into the buffer in little-endian order.
		buf[i*4+0] = byte(left)
		buf[i*4+1] = byte(left >> 8)
		buf[i*4+2] = byte(right)
		buf[i*4+3] = byte(right >> 8)
	}
	// Now update your visualization with the new PCM data.
	updateVisualization(buf)
	return n, ok
}

// Err just passes through the error from the wrapped streamer.
func (t *TapStreamer) Err() error {
	return t.s.Err()
}
