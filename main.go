// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"log"
	"os"
)

type C = layout.Context
type D = layout.Dimensions

var openButton, backButton, fwdButton, playButton, stopButton widget.Clickable
var volumeSlider widget.Float // widget state for the slider

// Channel to signal when the UI is ready
var uiReadyChan = make(chan struct{})

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("QuickClip"))
		w.Option(app.Size(unit.Dp(800), unit.Dp(400)))
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
	// This is important to allow the interface to show up before being blocked on WASM clients
	// so they can interact with the page and unblock the audio context
	<-uiReadyChan
	initializeOtoCtx()
	app.Main()

}

// Launch Gio rendering loop
func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	volumeSlider.Value = float32(playbackVolume)
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
			if volumeSlider.Update(gtx) {
				if currentPlayer != nil {
					currentPlayer.SetVolume(float64(volumeSlider.Value))
				}
				playbackVolume = float64(volumeSlider.Value)
			}
			render(gtx, th, evt)
		}
	}
}
