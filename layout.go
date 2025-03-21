package main

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
)

var fileDialog *explorer.Explorer
var openButton, backButton, fwdButton, playButton, stopButton widget.Clickable
var volumeSlider widget.Float // widget state for the slider
var playbackProgress float32

type C = layout.Context
type D = layout.Dimensions

func openFileDialog(w *app.Window) {
	if fileDialog == nil {
		fileDialog = explorer.NewExplorer(w)
	}

	// Open file dialog for a single audio file
	reader, err := fileDialog.ChooseFile(".wav", ".flac", ".mp3")
	if err != nil {
		log.Println("Error selecting file:", err)
		return
	}

	eject()
	currentReader = reader
	play(w) // keep playing with new reader
}

func updateProgressBar(pUnit *playbackUnit) {
	playbackProgress = pUnit.getProgressFloat()
}

func resetProgressBar() {
	playbackProgress = 0
}

func render(gtx layout.Context, th *material.Theme, e app.FrameEvent) {
	// Draw dark gray background.
	paint.ColorOp{Color: color.NRGBA{R: 30, G: 30, B: 30, A: 255}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	spacing := unit.Dp(5)

	// Outer horizontal flex: left for waveform/progress, right for buttons.
	layout.Flex{
		Axis:    layout.Horizontal,
		Spacing: layout.SpaceStart,
	}.Layout(gtx,
		// Left column: waveform on top, progress bar at bottom.
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceStart,
			}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return renderWaveform(gtx, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
				}),
				layout.Rigid(func(gtx C) D { // Mid buttons
					return layout.Flex{
						Axis:    layout.Horizontal,
						Spacing: layout.SpaceSides,
					}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.X = gtx.Dp(150)
							return material.Button(th, &openButton, "Open").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: spacing}.Layout),
						layout.Rigid(func(gtx C) D {
							if currentState == Playing {
								return material.Button(th, &stopButton, "Stop").Layout(gtx)
							}
							return material.Button(th, &playButton, "Play").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: spacing}.Layout),
						layout.Rigid(func(gtx C) D {
							return material.Button(th, &backButton, "Back").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: spacing}.Layout),
						layout.Rigid(func(gtx C) D {
							return material.Button(th, &fwdButton, "Forward").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: spacing}.Layout),
						layout.Rigid(func(gtx C) D {
							slider := material.Slider(th, &volumeSlider) // Default value set in Main
							gtx.Constraints.Min.X = gtx.Dp(150)
							gtx.Constraints.Max.X = gtx.Dp(150)
							return slider.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(5), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx C) D {
						const progressBarHeight = 10
						gtx.Constraints.Min.Y = gtx.Dp(progressBarHeight)
						gtx.Constraints.Max.Y = gtx.Dp(progressBarHeight)
						return material.ProgressBar(th, playbackProgress).Layout(gtx)
					})
				}),
			)
		}),
	)

	e.Frame(gtx.Ops)
}
