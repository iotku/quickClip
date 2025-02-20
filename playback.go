package main

import (
	"gioui.org/app"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"io"
	"log"
	"time"
)

var currentReader io.Reader
var currentPlayer *oto.Player // Track the current player
var globalOtoCtx *oto.Context

const bufferSize = 44100 * 2 * 2 // 1 second of audio at 44.1kHz
var audioRingBuffer = make([]byte, bufferSize)
var ringWritePos = 0
var playbackTime float64 = 0
var playbackVolume float64 = 0.7 // initial playbackVolume 70%

type PlaybackState int

const (
	NotInitialized PlaybackState = iota
	Playing
	Suspended
	Finished
)

var currentState PlaybackState = NotInitialized

func intializeOtoCtx() {
	if globalOtoCtx != nil {
		return
	}
	// Initialize the global Oto context once here.
	// Note: Choose options that work for your app.
	// If you later need a different sample rate (e.g., from an MP3),
	// you might need to convert or resample, because reinitializing
	// is not allowed.
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
		log.Println("No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	reader := currentReader
	// Decode the MP3 file.
	decodedMp3, err := mp3.NewDecoder(reader)
	if err != nil {
		log.Println("mp3.NewDecoder failed:", err)
		return
	}

	// Wrap the decoder with a TeeReader. The TeeReader will write all data
	// that is read by the player into a buffer that we can read from for visualization.
	// For simplicity, we'll use a channel to pass chunks of data.
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
	ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	defer ticker.Stop()

	for player.IsPlaying() {
		select {
		case chunk := <-visualCh:
			// Use the latest chunk for visualization.
			updateVisualization(chunk)
			w.Invalidate()
		case <-ticker.C:
			// Even if no new chunk is available, force a redraw.
			w.Invalidate()
		}
	}
	if err = player.Close(); err != nil {
		panic("player.Close failed: " + err.Error())
	}
	resetVisualization()
	currentState = Finished
}
