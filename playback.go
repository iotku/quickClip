package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
)

var currentReader io.Reader
var currentPlayer *oto.Player // Track the current player
var globalOtoCtx *oto.Context

const bufferSize = 44100 * 2 * 2 // 1 second of STEREO audio at 44.1kHz
var audioRingBuffer = make([]byte, bufferSize)
var ringWritePos = 0
var playbackTime = 0.0
var playbackVolume = 0.7 // initial playbackVolume 70%

// PlaybackState contains the various possible states of our playback
type PlaybackState int

const (
	NotInitialized PlaybackState = iota
	Playing
	Suspended
	Finished
)

var currentState PlaybackState = NotInitialized

func initializeOtoCtx() {
	if globalOtoCtx != nil {
		return
	}
	// Initialize the global Oto context once (!)
	// reinitializing the global context is not allowed and will PANIC
	opts := &oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	}

	ctx, readyChan, err := oto.NewContext(opts)
	if err != nil {
		panic("oto.NewContext failed: " + err.Error())
	}
	<-readyChan // Wait for the context to be ready

	globalOtoCtx = ctx
}

// getOtoContext returns the global oto context, creating it if necessary.
func getOtoContext() *oto.Context {
	if globalOtoCtx == nil {
		log.Println("GetOtoContext not initialized!!!")
	}
	return globalOtoCtx
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("playAudio: No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	reader := currentReader
	var fileExt string
	if file, ok := reader.(*os.File); ok {
		fileExt = filepath.Ext(file.Name())
		log.Println("Found", fileExt, "extension.")
	} else {
		log.Println("No valid file extension found.")
		return
	}

	switch fileExt {
	case ".mp3":
		log.Println("Using mp3 decoder")
	case ".wav":
		log.Println("Using wav decoder")
	default:
		log.Println("No decoder available for", fileExt)
	}

	decodedMp3, err := mp3.NewDecoder(reader)
	if err != nil {
		log.Println("mp3.NewDecoder failed:", err)
		return
	}

	// The TeeReader will write all data that is read by the player into a buffer
	// that we can read from for visualization sent via the visualCh channel.
	//
	// This way we avoid disrupting the mp3 decoding by working on a copy.
	visualCh := make(chan []byte, 10)
	tee := io.TeeReader(decodedMp3, newChunkWriter(visualCh))

	// Use the global Oto context. // NOTE: WE HAVE HARD CODED OPTIONS !
	otoCtx := getOtoContext()

	// Create a player that plays from the TeeReader.
	player := otoCtx.NewPlayer(tee)
	player.SetVolume(playbackVolume)
	player.Play()
	currentPlayer = player
	currentState = Playing

	// Visualization update loop: update at a fixed 60 FPS.
	// TODO: Possible to get monitor refresh rate or VSYNC at 60?
	ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	defer ticker.Stop()

	for player.IsPlaying() {
		select {
		case chunk := <-visualCh:
			// Use the latest chunk for visualization.
			updateVisualization(chunk)
			w.Invalidate()
		case <-ticker.C: // Force redraw at ticker interval
			w.Invalidate()
		}
	}
	if err = player.Close(); err != nil {
		panic("player.Close failed: " + err.Error())
	}
	resetVisualization()
	currentState = Finished
}
