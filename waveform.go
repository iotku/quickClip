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
var delaySeconds = .80

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
	// Early exit if there isn't enough audio data.
	if len(audioRingBuffer) < 2 {
		return layout.Dimensions{}
	}

	reduce := 4
	numSamples := width / reduce // 1/4 sample per pixel TODO: Expose as performance setting
	numBytes := numSamples * 2
	if len(audioRingBuffer) < numBytes {
		return layout.Dimensions{}
	}

	sampleRate := 44100
	offsetSamples := int(delaySeconds * float64(sampleRate))
	offsetBytes := offsetSamples * 2

	// Start a few samples earlier than the latest sample.
	startIndex := (ringWritePos + bufferSize - numBytes - offsetBytes) % bufferSize

	// Handle potential wrap-around by splitting the read if necessary.
	var samples []int16
	if startIndex+numBytes <= len(audioRingBuffer) {
		samples = bytesToInt16Slice(audioRingBuffer[startIndex : startIndex+numBytes])
	} else {
		// When the slice wraps around, split it into two parts and combine.
		firstPart := audioRingBuffer[startIndex:]
		secondPart := audioRingBuffer[:numBytes-len(firstPart)]
		combined := append(firstPart, secondPart...)
		samples = bytesToInt16Slice(combined)
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

	// Pre-calculate drawing parameters
	maxHeight := float32(height) / 2
	step := float32(width) / float32(numSamples)
	centerY := float32(height) / 2

	// Contrast parameters to make waveform more distinct
	exponent := float32(10.)
	alpha := float32(0.25)

	// Draw a static center line.
	var centerLinePath clip.Path
	centerLinePath.Begin(gtx.Ops)
	centerLinePath.MoveTo(f32.Pt(0, centerY))
	centerLinePath.LineTo(f32.Pt(float32(width), centerY))
	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		clip.Stroke{
			Path:  centerLinePath.End(),
			Width: 1,
		}.Op())

	// Build the waveform
	var path clip.Path
	path.Begin(gtx.Ops)

	if len(smoothedSamples) != numSamples {
		smoothedSamples = make([]float32, numSamples)
	}

	// First, update smoothedSamples from the raw samples.
	for i, s := range samples {
		dbMin := -40.0 // Silence threshold
		sampleFloat := float64(s) / float64(maxAmp)

		// Convert to dB, ensuring no log(0) issues
		db := 20 * math.Log10(math.Max(1e-5, math.Abs(sampleFloat)))

		// Normalize dB scale, ensuring silence stays at zero
		normalized := float32((db - dbMin) / (-dbMin))

		// Clamp values to avoid unwanted visual expansion
		if db <= dbMin {
			normalized = 0
		}

		contrasted := applyContrast32(normalized, exponent)
		scaled := contrasted * maxHeight
		smoothedSamples[i] = smoothedSamples[i]*(1-alpha) + scaled*alpha

		// Draw lines for each sample of the waveform
		x := float32(i) * step
		path.MoveTo(f32.Pt(x, centerY-smoothedSamples[i]))
		path.LineTo(f32.Pt(x, centerY+(smoothedSamples[i])))
	}

	path.Close()

	// Fill in the waveform.
	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 255, G: 0, B: 0, A: 255},
		clip.Stroke{
			Path:  path.End(),
			Width: float32(reduce),
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
func applyContrast32(normalized, exponent float32) float32 {
	if normalized >= 0 {
		return float32(math.Pow(float64(normalized), float64(exponent)))
	}
	return -float32(math.Pow(float64(-normalized), float64(exponent)))
}

// Aggressively cast []byte into a []int16 view (unsafe!)
func bytesToInt16Slice(b []byte) []int16 {
	n := len(b) / 2
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(unsafe.Pointer(&b[0])), n) // Risk it for the biscuit
}
