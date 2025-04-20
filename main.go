// SPDX-License-Identifier: MIT

package main

import (
	"gioui.org/io/pointer"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Channel to signal when the UI is ready
var uiReadyChan = make(chan struct{})

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("QuickClip"))
		w.Option(app.Size(unit.Dp(800), unit.Dp(400)))

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
	initSpeaker()
	app.Main()
}

// Launch Gio rendering loop
func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Fg = color.NRGBA{R: 255, G: 255, B: 255, A: 255} // White foreground text
	th.Bg = color.NRGBA{R: 30, G: 30, B: 30, A: 255}    // dark gray background
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
				if currentState == NotInitialized || currentState == Finished {
					go openFileDialog(w)
				} else {
					play(w)
				}
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

			if showDialog.Pressed() {
				w.Invalidate() // Show dialog immediately even if waveform isn't invalidating during playback
			}

			event, _ := gtx.Event(
				pointer.Filter{
					Target: &progressClickable,
					Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
				},
			)
			if progressBarEvt, ok := event.(pointer.Event); ok {
				windowWidth := gtx.Constraints.Max.X - 10 // NOTE: we subtract left/right layout insets TODO: remove magic number
				// Ratio of the progress bar to the window position (e.g. percentage through the bar from 0 to 1)
				// Note that the progressBarEvt position is relative to the WIDGET not the overall window
				ratioPos := progressBarEvt.Position.X / float32(windowWidth)

				switch progressBarEvt.Kind {
				case pointer.Press:
					isManualSeeking = true
					manualSeekPosition = ratioPos
				case pointer.Drag:
					manualSeekPosition = ratioPos
				case pointer.Release: // TODO: doesn't always fire when leaving window, Leave evt fixes this but bad UX
					isManualSeeking = false
					err := currentUnit.seekFloat(ratioPos)
					if err != nil {
						log.Println("seekFloat error:", err)
					}
					manualSeekPosition = ratioPos
					playbackProgress = ratioPos
				case pointer.Cancel: // user switched windows before release
					isManualSeeking = false
				default:
					log.Println("Unknown pointer event", event)
				}

			}

			render(gtx, th, evt)
		}
	}
}
