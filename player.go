package main

import (
	"log"

	"gioui.org/app"
)

func play(w *app.Window) {
	// Ensure the audio context is resumed immediately on a user gesture.
	if currentState == Suspended {
		currentUnit.setPaused(false)
		currentState = Playing
	} else {
		go playAudio(w)
	}
}

func stop() {
	if currentState != NotInitialized && currentState != Finished {
		currentUnit.setPaused(true)
		currentState = Suspended
	}
}

func eject() {
	// Stop playback if it's ongoing
	if currentState == Playing || currentState == Suspended {
		log.Println("Currently playing or suspended, EJECTING")
		stop()
	}

	// TODO: Does the old playback unit actually get garbage collected?
	//       I suspect it just remains paused in memory...
	
	// Reset any other relevant state
	currentState = NotInitialized
	playbackTime = 0
	resetVisualization() // Reset any ongoing visualization updates

	// Log out that the file has been ejected
	log.Println("Ejected current file and reset state.")
}
