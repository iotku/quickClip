// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"fmt"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"image"
	"image/color"
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

func playAudio(w *app.Window) {
	// Read the mp3 file into memory
	file, err := os.Open("./my-file.mp3")
	if err != nil {
		panic("opening my-file.mp3 failed: " + err.Error())
	}

	// Decode file. This process is done as the file plays so it won't
	// load the whole thing into memory.
	decodedMp3, err := mp3.NewDecoder(file)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}

	// Prepare an Oto context (this will use your default audio device) that will
	// play all our sounds. Its configuration can't be changed later.

	op := &oto.NewContextOptions{
		SampleRate:   44100,                   // Set sample rate for playback
		ChannelCount: 2,                       // Stereo
		Format:       oto.FormatSignedInt16LE, // 16-bit signed little-endian PCM
	}

	// Remember that you should **not** create more than one context THIS WILL PANIC
	otoCtx, readyChan, err := oto.NewContext(op)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	// It might take a bit for the hardware audio devices to be ready, so we wait on the channel.
	<-readyChan

	// Create a new 'player' that will handle our sound. Paused by default.
	player := otoCtx.NewPlayer(decodedMp3)

	// Play starts playing the sound and returns without waiting for it (Play() is async).
	player.Play()
	// Buffer to hold audio samples for visualization
	buffer := make([]byte, 16) // Adjust size as needed
	for player.IsPlaying() {
		// Fill the buffer with audio samples
		n, err := decodedMp3.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			log.Printf("Failed to decode audio: %v", err)
			break
		}

		// Update audioData with the decoded samples for visualization
		updateVisualization(buffer[:n]) // Only send the filled part of the buffer
		w.Invalidate()                  // Request a redraw of the window
		time.Sleep(time.Millisecond * 16)
	}

	// Now that the sound finished playing, we can restart from the beginning (or go to any location in the sound) using seek
	// newPos, err := player.(io.Seeker).Seek(0, io.SeekStart)
	// if err != nil{
	//     panic("player.Seek failed: " + err.Error())
	// }
	// println("Player is now at position:", newPos)
	// player.Play()

	// If you don't want the player/sound anymore simply close
	err = player.Close()
	if err != nil {
		panic("player.Close failed: " + err.Error())
	}
	// Once finished, reset the visualization
	resetVisualization()
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

	bars := audioData
	if len(audioData) < numBars {
		bars = make([]byte, numBars)
		copy(bars, audioData)
	}

	barWidth := width / numBars
	centerY := height / 2 // Middle of visualization

	alpha := 0.5 // Smoothing factor

	for i := 0; i < numBars; i++ {
		// Normalize byte to range [0, 1]
		sample := float32(bars[i]) / 255.0

		// Apply power function for contrast
		targetHeight := float32(math.Pow(float64(sample), 5)) * float32(centerY)

		// Apply smoothing
		smoothedBars[i] = smoothedBars[i]*(1-float32(alpha)) + targetHeight*float32(alpha)

		// Define mirrored bar (above & below center)
		rect := image.Rect(i*barWidth, centerY-int(smoothedBars[i]), (i+1)*barWidth, centerY+int(smoothedBars[i]))

		// Draw the bar
		stack := clip.Rect(rect).Push(gtx.Ops)
		paint.ColorOp{Color: color.NRGBA{R: 255, G: 0, B: 0, A: 255}}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		stack.Pop()
	}

	return layout.Dimensions{Size: image.Point{X: width, Y: height}}
}

func resetVisualization() {
	// Reset the visualization when the audio finishes (you can choose to clear the data here)
	audioData = nil
}
