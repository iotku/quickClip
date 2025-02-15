// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"bytes"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
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
				go playAudio()
			}
			render(gtx, th, ops, e)
		}
	}
}

func playAudio() {
	// Read the mp3 file into memory
	fileBytes, err := os.ReadFile("./my-file.mp3")
	if err != nil {
		panic("reading my-file.mp3 failed: " + err.Error())
	}

	// Convert the pure bytes into a reader object that can be used with the mp3 decoder
	fileBytesReader := bytes.NewReader(fileBytes)

	// Decode file
	decodedMp3, err := mp3.NewDecoder(fileBytesReader)
	if err != nil {
		panic("mp3.NewDecoder failed: " + err.Error())
	}

	// Prepare an Oto context (this will use your default audio device) that will
	// play all our sounds. Its configuration can't be changed later.

	op := &oto.NewContextOptions{}

	// Usually 44100 or 48000. Other values might cause distortions in Oto
	op.SampleRate = 44100

	// Number of channels (aka locations) to play sounds from. Either 1 or 2.
	// 1 is mono sound, and 2 is stereo (most speakers are stereo).
	op.ChannelCount = 2

	// Format of the source. go-mp3's format is signed 16bit integers.
	op.Format = oto.FormatSignedInt16LE

	// Remember that you should **not** create more than one context
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

	// We can wait for the sound to finish playing using something like this
	for player.IsPlaying() {
		time.Sleep(time.Millisecond)
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
}
func render(gtx layout.Context, th *material.Theme, ops op.Ops, e app.FrameEvent) {
	spacing := 5
	layout.Flex{
		// Vertical alignment, from top to bottom
		Axis: layout.Horizontal,
		// Empty space is left at the start, i.e. at the top
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
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
