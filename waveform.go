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
var waveformColor1 = color.NRGBA{R: 0, G: 255, B: 0, A: 255}
var waveformColor2 = color.NRGBA{R: 0, G: 0, B: 255, A: 255}

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
	// Early exit if there isn't enough audio data.
	if len(audioRingBuffer) < 2 {
		return layout.Dimensions{}
	}

	reduce := 4
	if isHqMode.Value {
		reduce = 1
	}
	numSamples := width / reduce // 1/4 sample per pixel TODO: Expose as performance setting
	numBytes := numSamples * 2
	if len(audioRingBuffer) < numBytes {
		return layout.Dimensions{}
	}

	// Where to start inside the ringBuffer
	startIndex := (ringWritePos + bufferSize - numBytes) % bufferSize

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
		color.NRGBA{R: 0, G: 0, B: 0, A: 255},
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

	// divide by factor to fit within height of the component
	var aReduceFactor float32 = 32767

	// First, update smoothedSamples from the raw samples.
	for i, s := range samples {
		dbMin := -120.0 // Silence threshold
		sampleFloat := float64(s) / float64(aReduceFactor)

		// Convert to dB, ensuring no log(0) issues
		db := 20 * math.Log10(math.Max(1e-5, math.Abs(sampleFloat)))

		normalized := float32((db - dbMin) / (-dbMin))

		contrasted := applyContrast32(normalized, exponent)
		scaled := contrasted * maxHeight
		smoothedSamples[i] = smoothedSamples[i]*(1-alpha) + scaled*alpha

		// Draw lines for each sample of the waveform
		x := float32(i) * step
		path.MoveTo(f32.Pt(x, centerY-smoothedSamples[i]))
		path.LineTo(f32.Pt(x, centerY+(smoothedSamples[i])))
	}

	path.Close()

	strokeOp := clip.Stroke{
		Path:  path.End(),
		Width: step,
	}.Op()

	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		strokeOp,
	)

	// Push same path a clipping region for colorization
	clipStack := strokeOp.Push(gtx.Ops)
	defer clipStack.Pop() // Ensure the clip is popped after drawing.

	// Draw gradient on top of waveform
	grad := paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Stop2:  f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)),
		Color1: waveformColor1,
		Color2: waveformColor2,
	}
	grad.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Point{X: width, Y: height}}
}

func updateVisualization(data []byte) {
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

	// Set all smoothedSamples to 0, we don't use nil here to avoid nil dereference
	for i := range smoothedSamples {
		smoothedSamples[i] = 0
	}
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
