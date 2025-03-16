// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"github.com/gopxl/beep/v2/speaker"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
)

type C = layout.Context
type D = layout.Dimensions

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

	// Wait for UI to be ready before initializing the audio ctx so they can interact first with the UI
	// This is critical to allow the interface to show up before being blocked on WASM clients
	<-uiReadyChan

	// NOTE: fixed buffer size for wasm MUST be divisible by 2
	err := speaker.Init(44100, 8194)
	if err != nil {
		log.Fatalln("Speaker INIT failed!:", err)
		return
	}
	app.Main()

}

// Launch Gio rendering loop
func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	volumeSlider.Value = float32(playbackVolume) // INITIAL VOLUME
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
			if fwdButton.Clicked(gtx) {
				forward()
			}
			if backButton.Clicked(gtx) {
				back()
			}
			if volumeSlider.Update(gtx) {
				currentUnit.setVolume(volumeSlider.Value)
			}
			render(gtx, th, evt)
		}
	}
}
