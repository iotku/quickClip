package main

import (
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"image"
	"image/color"
	"math"
	"unsafe"
)

var smoothedSamples []float32

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
	// Early exit if there isn't enough audio data.
	if len(audioRingBuffer) < 2 {
		return layout.Dimensions{}
	}

	sampleRate := 44100
	numSamples := width / 6 // 1/6 sample per pixel TODO: Expose as performance setting

	// Determine the starting sample based on playback time.
	startSample := int((playbackTime - audioLatencyOffset) * float64(sampleRate))
	startIndex := (startSample * 2) % bufferSize
	if startIndex < 0 {
		startIndex += bufferSize
	}

	// Ensure startIndex is even.
	if startIndex%2 != 0 {
		startIndex++
	}

	// Determine the number of bytes needed.
	numBytes := numSamples * 2
	var samples []int16

	if startIndex+numBytes <= len(audioRingBuffer) {
		// Get a direct view into the buffer.
		samples = bytesToInt16Slice(audioRingBuffer[startIndex : startIndex+numBytes])
	}

	// Determine the maximum amplitude.
	var maxAmp float32 = 1
	for _, s := range samples {
		amp := float32(s)
		if amp < 0 {
			amp = -amp
		}
		if amp > maxAmp {
			maxAmp = amp
		}
	}

	// Pre-calculate drawing parameters as float32.
	maxHeight := float32(height) / 2
	step := float32(width) / float32(numSamples)
	centerY := float32(height) / 2

	// Use float32 contrast parameters.
	exponent := float32(2.0)
	threshold := float32(0.15)

	alpha := float32(0.25)

	var path clip.Path
	path.Begin(gtx.Ops)

	// Ensure smoothedSamples is allocated.
	if len(smoothedSamples) != numSamples {
		smoothedSamples = make([]float32, numSamples)
	}
	// First, update smoothedSamples from the raw samples.
	for i, s := range samples {
		normalized := float32(s) / maxAmp
		if abs32(normalized) < threshold {
			normalized = 0
		}
		contrasted := applyContrast32(normalized, exponent)
		scaled := contrasted * maxHeight
		smoothedSamples[i] = smoothedSamples[i]*(1-alpha) + scaled*alpha
		x := float32(i) * step
		path.MoveTo(f32.Pt(x, centerY-smoothedSamples[i]))
		path.LineTo(f32.Pt(x, centerY+(smoothedSamples[i])))
	}

	path.Close()

	// Draw the waveform.
	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		clip.Stroke{
			Path:  path.End(),
			Width: 2,
		}.Op())

	return layout.Dimensions{Size: image.Point{X: width, Y: height}}
}

func updateVisualization(data []byte) {
	frameDuration := float64(len(data)) / float64(bufferSize) // 16-bit stereo
	playbackTime += frameDuration

	// Ensure we wrap around correctly
	copy(audioRingBuffer[ringWritePos:], data)

	// Handle wraparound case
	if ringWritePos+len(data) > len(audioRingBuffer) {
		remaining := (ringWritePos + len(data)) - len(audioRingBuffer)
		copy(audioRingBuffer[:remaining], data[len(data)-remaining:])
	}

	ringWritePos = (ringWritePos + len(data)) % len(audioRingBuffer)
}

func resetVisualization() {
	// Reset the audioRingBuffer to clear out any old audio data
	audioRingBuffer = make([]byte, bufferSize)

	// Reset the ring write position to the beginning
	ringWritePos = 0

	// Reset playback time to 0 to start fresh
	playbackTime = 0

	// Clear out any previously smoothed sample data
	smoothedSamples = nil
}

// applyContrast applies a power function to increase contrast.
// For positive values: result = normalized^exponent,
// for negative values: result = -(|normalized|^exponent).
func applyContrast32(normalized, exponent float32) float32 {
	if normalized >= 0 {
		return float32(math.Pow(float64(normalized), float64(exponent)))
	}
	return -float32(math.Pow(float64(-normalized), float64(exponent)))
}

// Risk it for the biscuit
func bytesToInt16Slice(b []byte) []int16 {
	n := len(b) / 2
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(unsafe.Pointer(&b[0])), n)
}
