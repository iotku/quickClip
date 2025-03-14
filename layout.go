package main

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
)

var fileDialog *explorer.Explorer // Initialized in Main

func openFileDialog(w *app.Window) {
	if fileDialog == nil {
		return
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
				// TODO: Track progress with progress bar bellow waveform
				// Progress bar: fixed height.
				//layout.Rigid(func(gtx C) D {
				//	// Wrap in an inset for a bit of padding.
				//	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx C) D {
				//		const progressBarHeight = 10
				//		gtx.Constraints.Min.Y = gtx.Dp(progressBarHeight)
				//		gtx.Constraints.Max.Y = gtx.Dp(progressBarHeight)
				//		progress := float32(0.5) // Example progress value.
				//		return material.ProgressBar(th, progress).Layout(gtx)
				//	})
				//}),
			)
		}),
		// Right column
		layout.Rigid(func(gtx C) D {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceStart,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return material.Button(th, &openButton, "Open").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spacing}.Layout),
				layout.Rigid(func(gtx C) D {
					return material.Button(th, &backButton, "Back").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spacing}.Layout),
				layout.Rigid(func(gtx C) D {
					return material.Button(th, &fwdButton, "Forward").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spacing}.Layout),
				layout.Rigid(func(gtx C) D {
					if currentState == Playing {
						return material.Button(th, &stopButton, "Stop").Layout(gtx)
					}
					return material.Button(th, &playButton, "Play").Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					slider := material.Slider(th, &volumeSlider) // Default value set in Main
					gtx.Constraints.Min.X = gtx.Dp(150)
					gtx.Constraints.Max.X = gtx.Dp(150)
					return slider.Layout(gtx)
				}),
			)
		}),
	)

	e.Frame(gtx.Ops)
}
