package main

import (
	"github.com/gopxl/beep/v2/speaker"
	"log"

	"gioui.org/app"
)

func play(w *app.Window) {
	// Ensure the audio context is resumed immediately on a user gesture.
	if currentState == Suspended {
		speaker.Lock()
		currentCtrl.Paused = false
		currentState = Playing
		speaker.Unlock()
	} else {
		go playAudio(w)
	}
}

func stop() {
	if currentState != NotInitialized && currentState != Finished {
		speaker.Lock()
		currentCtrl.Paused = true
		speaker.Unlock()
		currentState = Suspended
	}
}

func eject() {
	// Stop playback if it's ongoing
	if currentState == Playing || currentState == Suspended {
		log.Println("Currently playing or suspended, EJECTING")
		stop()
	}

	// Optionally: Reset any other relevant state
	currentState = NotInitialized
	playbackTime = 0
	resetVisualization() // Reset any ongoing visualization updates

	// Log out that the file has been ejected
	log.Println("Ejected current file and reset state.")
}
