package main

import (
	"gioui.org/app"
	"log"
	"time"
)

func play(w *app.Window) {
	// Ensure the audio context is resumed immediately on a user gesture.
	if globalOtoCtx != nil {
		globalOtoCtx.Resume()
	}

	if currentState == Suspended {
		currentState = Playing
	} else {
		// Schedule playAudio after a very short delay  // work around WASM bugs
		time.AfterFunc(5*time.Millisecond, func() {
			go playAudio(w)
		})
	}
}

func stop() {
	if currentState != NotInitialized && currentState != Finished {
		globalOtoCtx.Suspend()
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
