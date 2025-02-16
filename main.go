// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"encoding/binary"
	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type C = layout.Context
type D = layout.Dimensions

var backButton, fwdButton, playButton, stopButton widget.Clickable
var audioData []byte

const bufferSize = 44100 * 2 * 2 // 1 second of audio at 44.1kHz
const audioLatencyOffset = 0.9   // Adjust this value as needed (in seconds)
var audioRingBuffer = make([]byte, bufferSize)
var ringWritePos = 0
var isPlaying = false
var playbackTime float64 = 0 // Global variable to track playback progress
// Global variable to store smoothed sample values.
var smoothedSamples []float32

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("QuickClip"))
		w.Option(app.Size(unit.Dp(400), unit.Dp(600)))
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if playButton.Clicked(gtx) && !isPlaying {
				isPlaying = true
				go playAudio(w)
			}
			render(gtx, th, ops, e)
		}
	}
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	// Open the mp3 file
	file, err := os.Open("./my-file.mp3")
	if err != nil {
		panic("opening my-file.mp3 failed: " + err.Error())
	}
	// Ensure file is closed eventually.
	defer file.Close()

	// Decode the MP3 file.
	decodedMp3, err := mp3.NewDecoder(file)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}

	// Wrap the decoder with a TeeReader. The TeeReader will write all data
	// that is read by the player into a buffer that we can read from for visualization.
	// For simplicity, we'll use a channel to pass chunks of data.
	visualCh := make(chan []byte, 10)
	tee := io.TeeReader(decodedMp3, newChunkWriter(visualCh))

	// Prepare an Oto context using the sample rate from the decoded MP3.
	otoOptions := &oto.NewContextOptions{
		SampleRate:   decodedMp3.SampleRate(),
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	otoCtx, readyChan, err := oto.NewContext(otoOptions)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan

	// Create a player that plays from the TeeReader.
	player := otoCtx.NewPlayer(tee)
	player.Play()

	// Visualization update loop: update at a fixed 60 FPS.
	ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	defer ticker.Stop()

	for player.IsPlaying() {
		select {
		case chunk := <-visualCh:
			// Use the latest chunk for visualization.
			updateVisualization(chunk)
			w.Invalidate()
		case <-ticker.C:
			// Even if no new chunk is available, force a redraw.
			w.Invalidate()
		}
	}
	if err = player.Close(); err != nil {
		panic("player.Close failed: " + err.Error())
	}
	resetVisualization()
	isPlaying = false
}

// chunkWriter implements io.Writer and sends written chunks over a channel.
type chunkWriter struct {
	ch chan<- []byte
}

func newChunkWriter(ch chan<- []byte) *chunkWriter {
	return &chunkWriter{ch: ch}
}

func (cw *chunkWriter) Write(p []byte) (n int, err error) {
	// Copy p into a new slice so it won't be modified later.
	buf := make([]byte, len(p))
	copy(buf, p)
	// Non-blocking send.
	select {
	case cw.ch <- buf:
	default:
	}
	return len(p), nil
}

func render(gtx layout.Context, th *material.Theme, ops op.Ops, e app.FrameEvent) {
	paint.ColorOp{Color: color.NRGBA{R: 30, G: 30, B: 30, A: 255}}.Add(gtx.Ops) // Dark gray background
	paint.PaintOp{}.Add(gtx.Ops)
	spacing := 5
	// Visualization size and padding
	//visualizationHeight := 800
	//visualizationWidth := 800

	layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Horizontal,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		layout.Rigid(
			func(gtx C) D {
				// Render the audio visualization
				if len(audioRingBuffer) > 0 {
					// Create a container for the waveform visualization
					return renderWaveform(gtx, gtx.Constraints.Max.X-260, gtx.Constraints.Max.Y) // TODO: Calculate based on button size...
				}
				return layout.Dimensions{}
			},
		),
		layout.Rigid(
			func(gtx C) D {
				btnBack := material.Button(th, &backButton, "Back")
				return btnBack.Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				btnFwd := material.Button(th, &fwdButton, "Forward")
				return btnFwd.Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				btnPlay := material.Button(th, &playButton, "Play")

				return btnPlay.Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				btnStop := material.Button(th, &stopButton, "Stop")
				return btnStop.Layout(gtx)
			},
		),
	)
	e.Frame(gtx.Ops)
}

// abs returns the absolute value of a float32.
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
	// Fix: Check for valid data
	if len(audioRingBuffer) < 2 {
		return layout.Dimensions{}
	}

	sampleRate := 44100
	numSamples := width // One sample per pixel

	// Find where to start rendering based on playbackTime
	startSample := int((playbackTime - audioLatencyOffset) * float64(sampleRate))
	startIndex := (startSample * 2) % bufferSize

	// Ensure the startIndex is non-negative
	if startIndex < 0 {
		startIndex += bufferSize
	}

	// Extract samples
	samples := make([]int16, numSamples)

	for i := 0; i < numSamples; i++ {
		// Ensure sampleIndex is non-negative
		sampleIndex := (startIndex + i*2) % bufferSize
		if sampleIndex < 0 {
			sampleIndex += bufferSize
		}

		// Ensure there are enough bytes to access
		if sampleIndex+1 < len(audioRingBuffer) {
			samples[i] = int16(binary.LittleEndian.Uint16(audioRingBuffer[sampleIndex : sampleIndex+2]))
		} else {
			// Handle the case where we don't have enough data yet
			samples[i] = 0
		}
	}

	// Find the maximum amplitude
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

	// Set up drawing parameters
	maxHeight := height / 2
	step := float32(width) / float32(numSamples)
	centerY := float32(height) / 2

	// Contrast parameters
	exponent := 2.0
	threshold := 0.05

	// Fix: Ensure smoothedSamples is properly initialized
	if len(smoothedSamples) != numSamples {
		smoothedSamples = make([]float32, numSamples)
	}

	// Smoothing factor
	alpha := float32(0.2)

	var path clip.Path
	path.Begin(gtx.Ops)

	// Process samples to compute smoothed amplitude
	for i, s := range samples {
		normalized := float32(s) / maxAmp
		if math.Abs(float64(normalized)) < float64(threshold) {
			normalized = 0
		}
		contrasted := applyContrast(normalized, exponent)
		scaled := contrasted * float32(maxHeight)
		smoothedSamples[i] = smoothedSamples[i]*(1-alpha) + scaled*alpha

		x := float32(i) * step
		y := centerY - smoothedSamples[i]
		if i == 0 {
			path.MoveTo(f32.Pt(x, y))
		} else {
			path.LineTo(f32.Pt(x, y))
		}
	}

	// Mirror lower half
	for i := len(samples) - 1; i >= 0; i-- {
		x := float32(i) * step
		y := centerY + smoothedSamples[i]
		path.LineTo(f32.Pt(x, y))
	}

	path.Close()

	// Draw the waveform
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 0, B: 0, A: 255}, clip.Stroke{
		Path:  path.End(),
		Width: 1,
	}.Op())

	log.Println("Start index:", startIndex, "Buffer size:", bufferSize)
	log.Println("Num samples:", numSamples)

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
	audioData = nil
}

// applyContrast applies a power function to increase contrast.
// For positive values: result = normalized^exponent,
// for negative values: result = -(|normalized|^exponent).
func applyContrast(normalized float32, exponent float64) float32 {
	if normalized >= 0 {
		return float32(math.Pow(float64(normalized), exponent))
	}
	return -float32(math.Pow(float64(-normalized), exponent))
}
