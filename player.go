package main

import (
	"log"

	"gioui.org/app"
)

func play(w *app.Window) {
	if currentState == Suspended { // Resume Paused playback
		currentUnit.setPaused(false)
		currentState = Playing
		return
	}
	go playAudio(w)
}

func stop() {
	if currentState != NotInitialized && currentState != Finished {
		currentUnit.setPaused(true)
		currentState = Suspended
	}
}

func eject() {
	if currentState == Playing || currentState == Suspended { // Stop ongoing playback
		log.Println("Currently playing or suspended, EJECTING")
		stop()
		currentUnit.done <- true
	}

	// TODO: Does the old playback unit actually get garbage collected?
	//       I suspect it just remains paused in memory...

	// Reset any other relevant state
	currentState = NotInitialized
	resetVisualization() // Reset any ongoing visualization updates

	log.Println("Ejected current file and reset state.")
}

func forward() {
	if err := currentUnit.forward(); err != nil {
		return
	}
	updateProgressBar(currentUnit)
}

func back() {
	if err := currentUnit.back(); err != nil {
		return
	}
	updateProgressBar(currentUnit)
}
