// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"fmt"
	"gioui.org/f32"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"image/color"
	"io"
	"log"
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

const numBars = 200

var smoothedBars [numBars]float32 // Stores smoothed bar heights

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

	var isPlaying bool = false
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
	op := &oto.NewContextOptions{
		SampleRate:   decodedMp3.SampleRate(),
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	otoCtx, readyChan, err := oto.NewContext(op)
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
	visualizationWidth := 800

	layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Horizontal,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		layout.Rigid(
			func(gtx C) D {
				// Render the audio visualization
				if len(audioData) > 0 {
					// Create a container for the waveform visualization
					return renderWaveform(gtx, visualizationWidth, gtx.Constraints.Max.Y)
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

func updateVisualization(data []byte) {
	// Append new audio data and maintain a fixed buffer size
	audioData = append(audioData, data...)

	// Keep only the most recent `numBars` samples
	if len(audioData) > numBars {
		audioData = audioData[len(audioData)-numBars:]
	}
	fmt.Println("Audio Data: ", audioData)
}

func renderWaveform(gtx layout.Context, width, height int) layout.Dimensions {
	if len(audioData) == 0 {
		return layout.Dimensions{}
	}

	maxHeight := height / 2 // Half of the window height for the center
	numSamples := len(audioData)

	// Path for the waveform
	var path clip.Path
	path.Begin(gtx.Ops)

	step := float32(width) / float32(numSamples) // Space between points
	centerY := float32(height / 2)

	// Find the maximum value in the audio data for normalization
	var maxSample float32
	for _, sample := range audioData {
		normalizedSample := (float32(sample) - 128) / 128.0
		if normalizedSample > maxSample {
			maxSample = normalizedSample
		}
	}

	// First half of the waveform (left side)
	for i, sample := range audioData {
		// Normalize sample (0 to 255) to range -1 to 1
		normalizedSample := (float32(sample) - 128) / 128.0

		// Normalize the sample based on the max sample
		scaledSample := normalizedSample / maxSample * float32(maxHeight)

		// Apply scaling to keep the waveform centered
		x := float32(i) * step
		y := centerY - scaledSample // Scale based on center

		if i == 0 {
			path.MoveTo(f32.Pt(x, y))
		} else {
			path.LineTo(f32.Pt(x, y))
		}
	}

	// Second half of the waveform (right side, mirrored)
	for i := len(audioData) - 1; i >= 0; i-- {
		normalizedSample := (float32(audioData[i]) - 128) / 128.0
		scaledSample := normalizedSample / maxSample * float32(maxHeight)

		x := float32(i) * step
		y := centerY + scaledSample // Continue from center without inverting

		path.LineTo(f32.Pt(x, y))
	}

	// Close the waveform path
	path.Close()

	// Draw the waveform with a stroke
	paint.FillShape(gtx.Ops, color.NRGBA{R: 255, G: 0, B: 0, A: 255}, clip.Stroke{
		Path:  path.End(),
		Width: 1, // Thickness of waveform line
	}.Op())

	return layout.Dimensions{Size: image.Point{X: width, Y: height}}
}

func resetVisualization() {
	// Reset the visualization when the audio finishes (you can choose to clear the data here)
	audioData = nil
}
