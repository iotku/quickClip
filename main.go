// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"encoding/binary"
	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/x/explorer"
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

var openButton, backButton, fwdButton, playButton, stopButton widget.Clickable
var fileDialog *explorer.Explorer // Initialized in Main
var currentReader io.Reader
var currentPlayer *oto.Player // Track the current player

// Channel to signal when the UI is ready
var uiReadyChan = make(chan struct{})

const bufferSize = 44100 * 2 * 2 // 1 second of audio at 44.1kHz
const audioLatencyOffset = 0.9   // Adjust this value as needed (in seconds) TODO: This is broken for values >= 1
var audioRingBuffer = make([]byte, bufferSize)
var ringWritePos = 0
var playbackTime float64 = 0
var smoothedSamples []float32

var globalOtoCtx *oto.Context

type PlaybackState int

const (
	NotInitialized PlaybackState = iota
	Playing
	Suspended
	Finished
)

var currentState PlaybackState = NotInitialized

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("QuickClip"))
		w.Option(app.Size(unit.Dp(800), unit.Dp(600)))
		fileDialog = explorer.NewExplorer(w)

		// Notify that the UI is ready
		close(uiReadyChan)
		// Start Render loop
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	// Wait for the UI to be ready before initializing Oto
	<-uiReadyChan
	intializeOtoCtx()
	app.Main()

}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var ops op.Ops
	for {
		e := w.Event()
		switch evt := e.(type) {
		case app.DestroyEvent:
			return evt.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, evt)
			if openButton.Clicked(gtx) {
				go openFileDialog(w)
			}
			if playButton.Clicked(gtx) {
				play(w)
			}
			if stopButton.Clicked(gtx) {
				stop()
			}
			render(gtx, th, ops, evt)
		}
	}
}

func intializeOtoCtx() {
	if globalOtoCtx != nil {
		return
	}
	// Initialize the global Oto context once here.
	// Note: Choose options that work for your app.
	// If you later need a different sample rate (e.g., from an MP3),
	// you might need to convert or resample, because reinitializing
	// is not allowed.
	opts := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, readyChan, err := oto.NewContext(opts)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan // Wait for the context to be ready

	globalOtoCtx = ctx
}
func openFileDialog(w *app.Window) {
	if fileDialog == nil {
		return
	}

	// Open file dialog for a single MP3 file
	reader, err := fileDialog.ChooseFile("mp3")
	if err != nil {
		log.Println("Error selecting file:", err)
		return
	}

	eject()
	currentReader = reader
	if currentState == Playing {
		play(w) // keep playing with new reader
	}
}

func eject() {
	// Stop playback if it's ongoing
	if currentState == Playing || currentState == Suspended {
		stop()
	}

	// Ensure that we close the player and release resources
	if currentPlayer != nil {
		err := currentPlayer.Close()
		if err != nil {
			log.Println("Error closing the player:", err)
		}
		currentPlayer = nil // Reset the player
	}

	// Optionally: Reset any other relevant state
	currentState = NotInitialized
	playbackTime = 0
	resetVisualization() // Reset any ongoing visualization updates

	// Log out that the file has been ejected
	log.Println("Ejected current file and reset state.")

}
func stop() {
	if currentState != NotInitialized && currentState != Finished {
		globalOtoCtx.Suspend()
		currentState = Suspended
	}
}

func play(w *app.Window) {
	// Ensure the audio context is resumed immediately on a user gesture.
	if globalOtoCtx != nil {
		globalOtoCtx.Resume()
	}

	if currentState == Suspended {
		currentState = Playing
	} else {
		// Schedule playAudio after a very short delay  // work around WASM bugs
		time.AfterFunc(5*time.Millisecond, func() {
			go playAudio(w)
		})
	}
}

// getOtoContext returns the global oto context, creating it if necessary.
func getOtoContext() *oto.Context {
	if globalOtoCtx == nil {
		log.Println("GetOtoContext not initialized!!!")
	}
	return globalOtoCtx
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	reader := currentReader
	// Decode the MP3 file.
	decodedMp3, err := mp3.NewDecoder(reader)
	if err != nil {
		log.Println("mp3.NewDecoder failed:", err)
		return
	}

	// Wrap the decoder with a TeeReader. The TeeReader will write all data
	// that is read by the player into a buffer that we can read from for visualization.
	// For simplicity, we'll use a channel to pass chunks of data.
	visualCh := make(chan []byte, 10)
	tee := io.TeeReader(decodedMp3, newChunkWriter(visualCh))

	// Use the global Oto context. // NOTE: WE HAVE HARD CODED OPTIONS !
	otoCtx := getOtoContext()

	// Create a player that plays from the TeeReader.
	player := otoCtx.NewPlayer(tee)
	player.Play()
	currentPlayer = player
	currentState = Playing

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
	currentState = Finished
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
					if gtx.Constraints.Max.X > 330 { // TODO: Remove magic number, must be large enough or will negative index
						return renderWaveform(gtx, gtx.Constraints.Max.X-330, gtx.Constraints.Max.Y) // TODO: Calculate based on button size...
					}
				}
				return layout.Dimensions{}
			},
		),
		layout.Rigid(
			func(gtx C) D {
				return material.Button(th, &openButton, "Open").Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				return material.Button(th, &backButton, "Back").Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				return material.Button(th, &fwdButton, "Forward").Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				return material.Button(th, &playButton, "Play").Layout(gtx)
			},
		),
		layout.Rigid(layout.Spacer{Width: unit.Dp(spacing)}.Layout),
		layout.Rigid(
			func(gtx C) D {
				return material.Button(th, &stopButton, "Stop").Layout(gtx)
			},
		),
	)
	e.Frame(gtx.Ops)
}

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
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
	exponent := 2.5
	threshold := 0.15

	// Ensure smoothedSamples is properly initialized
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
func applyContrast(normalized float32, exponent float64) float32 {
	if normalized >= 0 {
		return float32(math.Pow(float64(normalized), exponent))
	}
	return -float32(math.Pow(float64(-normalized), exponent))
}

// WASM workaround
func unlockAudioContext() {
	if globalOtoCtx != nil {
		// Resume the context immediately.
		globalOtoCtx.Resume()

		// Create a short silent buffer (e.g. 100ms of silence).
		silentDuration := 0.1 // in seconds
		sampleRate := 44100
		// For stereo 16-bit audio, each sample takes 4 bytes.
		numBytes := int(float64(sampleRate) * silentDuration * 4)
		silentBuffer := make([]byte, numBytes) // all zeros = silence

		// Create a player for the silent buffer.
		player := globalOtoCtx.NewPlayer(bytes.NewReader(silentBuffer))
		player.Play()

		// Let it play for a short while, then close.
		time.Sleep(100 * time.Millisecond)
		player.Close()
	}
}
