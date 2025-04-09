package main

import (
	"fmt"
	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/colorpicker"
	"gioui.org/x/explorer"
	"image"
	"image/color"
	"log"
	"runtime"
)

var fileDialog *explorer.Explorer
var openButton, backButton, fwdButton, playButton, stopButton widget.Clickable
var progressClickable widget.Clickable
var volumeSlider widget.Float // widget state for the slider
var playbackProgress float32
var isManualSeeking bool
var manualSeekPosition float32
var itemSpacing = unit.Dp(5)

var showDialog widget.Bool
var isHqMode widget.Bool

type C = layout.Context
type D = layout.Dimensions

func init() {
	isHqMode.Value = runtime.GOOS != "js" // Default to HQ mode on non-wasm
}

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
					return layout.Stack{}.Layout(gtx,
						layout.Expanded(func(gtx C) D {
							return renderWaveform(gtx, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
						}),
						layout.Stacked(func(gtx C) D {
							if showDialog.Value {
								return renderDialog(gtx, th)
							}
							return layout.Dimensions{}
						}),
					)
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
						layout.Rigid(layout.Spacer{Width: itemSpacing}.Layout),
						layout.Rigid(func(gtx C) D {
							if currentState == Playing {
								return material.Button(th, &stopButton, "Stop").Layout(gtx)
							}
							return material.Button(th, &playButton, "Play").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: itemSpacing}.Layout),
						layout.Rigid(func(gtx C) D {
							return material.Button(th, &backButton, "Back").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: itemSpacing}.Layout),
						layout.Rigid(func(gtx C) D {
							return material.Button(th, &fwdButton, "Forward").Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: itemSpacing}.Layout),
						layout.Rigid(func(gtx C) D {
							return material.CheckBox(th, &showDialog, "Options").Layout(gtx)
						}),
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
						return progressClickable.Layout(gtx, func(gtx C) D {
							return layout.Center.Layout(gtx, func(gtx C) D {
								gtx2 := gtx
								gtx2.Constraints.Min.Y = gtx.Dp(progressBarHeight)
								gtx2.Constraints.Max.Y = gtx.Dp(progressBarHeight)
								if isManualSeeking {
									return material.ProgressBar(th, manualSeekPosition).Layout(gtx2)
								}
								return material.ProgressBar(th, playbackProgress).Layout(gtx2)
							})
						})
					})
				}),
			)
		}),
	)

	e.Frame(gtx.Ops)
}

var mState1 colorpicker.State
var mState2 colorpicker.State
var ps1 colorpicker.PickerStyle
var ps2 colorpicker.PickerStyle

func init() {
	// TODO: Currently if we set the waveform color inside render dialog we can't type values
	// This definitely is not the proper method of setting up the color pickers...
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Fg = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	// TODO: currently theme visability is poor (!)
	ps1 = colorpicker.Picker(th, &mState1, "Left")
	ps2 = colorpicker.Picker(th, &mState2, "Right")
	ps1.SetColor(waveformColor1)
	ps2.SetColor(waveformColor2)
}

func renderDialog(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Draw a semi-transparent overlay background.
	paint.Fill(gtx.Ops, color.NRGBA{R: 0, G: 0, B: 0, A: 180})
	const width = 500
	const height = 270
	// Dialog position offset
	return layout.Inset{
		Left: unit.Dp(gtx.Constraints.Max.X/2 - width/2),
		Top:  unit.Dp(gtx.Constraints.Max.Y) - unit.Dp(height) - unit.Dp(itemSpacing),
	}.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Dp(width), gtx.Dp(height))
		rect := clip.RRect{ // Rounded rectangle for the dialog.
			Rect: image.Rectangle{Max: size},
			SE:   gtx.Dp(12), SW: gtx.Dp(12),
			NE: gtx.Dp(12), NW: gtx.Dp(12),
		}
		paint.FillShape(gtx.Ops, th.Bg, rect.Op(gtx.Ops)) // Dialog Background

		// Use an inset to provide inner margins.
		return layout.Inset{
			Top:    unit.Dp(20),
			Bottom: unit.Dp(20),
			Left:   unit.Dp(16),
			Right:  unit.Dp(16),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceEvenly,
			}.Layout(gtx,
				layout.Rigid(material.Body1(th, "Waveform Colors:").Layout),
				layout.Rigid(layout.Spacer{Height: itemSpacing}.Layout),
				// Side x Side color pickers
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					const totalWidth = width - (16 * 2) // dialog width - left/right inset
					return layout.Flex{
						Axis:    layout.Horizontal,
						Spacing: layout.SpaceBetween,
					}.Layout(gtx,
						// First color picker.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = gtx.Dp(totalWidth / 2)
							dims := ps1.Layout(gtx)
							waveformColor1 = ps1.State.Color()
							return dims
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Max.X = gtx.Dp(totalWidth / 2)
							dims := ps2.Layout(gtx)
							waveformColor2 = ps2.State.Color()
							return dims
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: itemSpacing}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(th, &isHqMode, "HQ Mode").Layout(gtx)
				}),
			)
		})
	})
}

func renderColorSquare(gtx layout.Context, col color.NRGBA) layout.Dimensions {
	boxSize := gtx.Dp(25)
	rect := image.Rect(0, 0, boxSize, boxSize)

	// Push the clip and defer the pop
	defer clip.Rect(rect).Push(gtx.Ops).Pop()

	paint.Fill(gtx.Ops, col)

	return layout.Dimensions{Size: image.Pt(boxSize, boxSize)}
}

func displayColorSquare(gtx layout.Context, th *material.Theme, color color.NRGBA) layout.Dimensions {
	return layout.Flex{
		Axis:    layout.Vertical,
		Spacing: layout.SpaceEvenly,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Horizontal,
				Spacing: layout.SpaceAround,
			}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return renderColorSquare(gtx, color) // Use passed-in color
				}),
				layout.Rigid(layout.Spacer{Width: itemSpacing}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					s := fmt.Sprintf("R: %d, G: %d, B: %d, A: %d", color.R, color.G, color.B, color.A)
					return material.Body2(th, s).Layout(gtx)
				}),
			)
		}),
	)
}
