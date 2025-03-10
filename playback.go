package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	//"os"
	//"path/filepath"
	"gioui.org/app"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
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

// Read magic bytes to determine what type of audio we have
func getAudioType(reader io.ReadCloser) (string, io.ReadCloser) {
	//if file, ok := reader.(*os.File); ok { // Just use the file extension
	//	fileExt := filepath.Ext(file.Name())
	//	log.Println("Found", fileExt, "extension.")
	//	return fileExt
	//} else { // read the magic bytes (for WASM)
	fileType, newRC, err := detectMagicBytes(reader)
	if err != nil {
		return "", nil
	}

	return fileType, newRC

	//}
}

// readCloserWrapper wraps an io.Reader and an io.Closer so that it satisfies io.ReadCloser.
type readCloserWrapper struct {
	io.Reader
	c io.Closer
}

func (rcw *readCloserWrapper) Close() error {
	return nil // no-op`
}

// detectMagicBytes reads the first 12 bytes to determine the file type,
// then returns a new io.ReadCloser that starts from byte position 0.
func detectMagicBytes(r io.ReadCloser) (string, io.ReadCloser, error) {
	// Read the first 12 bytes
	const headerSize = 12
	header := make([]byte, headerSize)
	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, fmt.Errorf("error reading magic bytes: %w", err)
	}
	header = header[:n]

	var fileType string
	switch {
	case len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE":
		fileType = ".wav"
	case len(header) >= 3 && string(header[:3]) == "ID3":
		fileType = ".mp3"
	case len(header) >= 2 && header[0] == 0xFF && (header[1]&0xF6) == 0xF2:
		fileType = ".mp3"
	default:
		log.Println("Could not determine audio type by magic bytes")
	}

	// Reconstruct a new ReadCloser that starts from the beginning:
	// Prepend the already-read header back onto the remaining stream.
	newReader := io.MultiReader(bytes.NewReader(header), r)
	return fileType, &readCloserWrapper{Reader: newReader, c: r}, nil
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("playAudio: No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	var audioStreamer beep.StreamSeekCloser
	var err error
	var format beep.Format
	audioType, newReader := getAudioType(currentReader)
	currentReader = newReader
	switch audioType {
	case ".mp3":
		log.Println("Using mp3 decoder")
		audioStreamer, format, err = mp3.Decode(currentReader)
	case ".wav":
		log.Println("Using wav decoder")
		audioStreamer, format, err = wav.Decode(currentReader)
	default:
		log.Println("No decoder available for", audioType)
		return
	}
	log.Println(format)

	if err != nil {
		log.Println("Decoder failed:", err)
		return
	}

	// Create audio pannel
	log.Println("Build loop streamer")
	loopStreamer, err := beep.Loop2(audioStreamer)
	if err != nil {
		log.Println("loop2 err:", err)
		return
	}

	ctrl := &beep.Ctrl{Streamer: loopStreamer}
	resampler := beep.ResampleRatio(4, 1, ctrl)
	//volume := &effects.Volume{Streamer: resampler, Base: 2}

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
	done := make(chan bool)
	currentState = Playing
	speaker.Play(beep.Seq(resampler, beep.Callback(func() {
		done <- true
	})))
	currentState = Finished
	// Visualization update loop: update at a fixed 60 FPS.
	// TODO: Possible to get monitor refresh rate or VSYNC at 60?
	//ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	//defer ticker.Stop()

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

}
