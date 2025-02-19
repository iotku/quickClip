package main

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"image/color"
	"log"
)

var fileDialog *explorer.Explorer // Initialized in Main

func openFileDialog(w *app.Window) {
	if fileDialog == nil {
		return
	}

	// Open file dialog for a single MP3 file
	reader, err := fileDialog.ChooseFile(".mp3")
	if err != nil {
		log.Println("Error selecting file:", err)
		return
	}

	lastState := currentState
	eject()
	currentReader = reader
	if lastState == Playing {
		play(w) // keep playing with new reader
	}
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
		layout.Flexed(1,
			func(gtx C) D {
				return renderWaveform(gtx, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
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
