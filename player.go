package main

import (
	"gioui.org/app"
	"log"
)

func play(w *app.Window) {
	// Ensure the audio context is resumed immediately on a user gesture.
	if globalOtoCtx != nil {
		err := globalOtoCtx.Resume()
		if err != nil {
			log.Println("Error resuming gio context:", err)
			return
		}
	}

	if currentState == Suspended {
		currentState = Playing
	} else {
		go playAudio(w)
	}
}

func stop() {
	if currentState != NotInitialized && currentState != Finished {
		err := globalOtoCtx.Suspend()
		if err != nil {
			log.Println("Error suspending gio context:", err)
			return
		}
		currentState = Suspended
	}
}

func eject() {
	// Stop playback if it's ongoing
	if currentState == Playing || currentState == Suspended {
		stop()
	}

	// Ensure that we close the player and release resources
	if currentPlayer != nil {
		err := currentPlayer.Close()
		if err != nil {
			log.Println("Error closing the player:", err)
		}
		currentPlayer = nil // Reset the player
	}

	// Optionally: Reset any other relevant state
	currentState = NotInitialized
	playbackTime = 0
	resetVisualization() // Reset any ongoing visualization updates

	// Log out that the file has been ejected
	log.Println("Ejected current file and reset state.")
}
