package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"gioui.org/app"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

var currentReader io.ReadCloser

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

type audioPanel struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeeker
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

func newAudioPanel(sampleRate beep.SampleRate, streamer beep.StreamSeeker) (*audioPanel, error) {
	loopStreamer, err := beep.Loop2(streamer)
	if err != nil {
		return nil, err
	}

	ctrl := &beep.Ctrl{Streamer: loopStreamer}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &audioPanel{sampleRate, streamer, ctrl, resampler, volume}, nil
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

	audioStreamer, format, err := mp3.Decode(currentReader)
	if err != nil {
		log.Println("mp3.NewDecoder failed:", err)
		return
	}
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/30))

	// Create audio pannel
	log.Println("Build loop streamer")
	loopStreamer, err := beep.Loop2(audioStreamer)
	if err != nil {
		log.Println(err)
		return
	}

	ctrl := &beep.Ctrl{Streamer: loopStreamer}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}

	// The TeeReader will write all data that is read by the player into a buffer
	// that we can read from for visualization sent via the visualCh channel.
	//
	// This way we avoid disrupting the original source
	// decoding by working on a copy instead.
	//visualCh := make(chan []byte, 10)
	//tee := io.TeeReader(audioDecoder, newChunkWriter(visualCh))

	// Create a player that plays from the TeeReader.
	//player.SetVolume(playbackVolume)
	//player.Play()
	log.Println("Play NOW")
	speaker.Play(volume)
	currentState = Playing

	// Visualization update loop: update at a fixed 60 FPS.
	// TODO: Possible to get monitor refresh rate or VSYNC at 60?
	ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	defer ticker.Stop()

	//for player.IsPlaying() {
	//		select {
	//		case chunk := <-visualCh:
	//			// Use the latest chunk for visualization.
	//			updateVisualization(chunk)
	//			w.Invalidate()
	//		case <-ticker.C: // Force redraw at ticker interval
	//			w.Invalidate()
	//		}
	//	}
	//	if err = player.Close(); err != nil {
	//		panic("player.Close failed: " + err.Error())
	//	}
	//	resetVisualization()
	currentState = Finished
}
