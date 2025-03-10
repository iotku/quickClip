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

func detectMagicBytes(r io.ReadSeeker) (string, error) {
	const headerSize = 12
	header := make([]byte, headerSize)

	// Read the first 12 bytes.
	n, err := io.ReadFull(r, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("error reading magic bytes: %w", err)
	}
	header = header[:n]

	// Determine file type from header.
	fileType := determineFileType(header)

	// Reset reader position.
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to reset reader position: %w", err)
	}

	return fileType, nil
}

func determineFileType(header []byte) string {
	switch {
	case len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WAVE":
		return ".wav"
	case len(header) >= 3 && string(header[:3]) == "ID3":
		return ".mp3"
	case len(header) >= 2 && header[0] == 0xFF && (header[1]&0xF6) == 0xF2:
		return ".mp3"
	default:
		log.Println("Could not determine audio type by magic bytes")
		return ""
	}
}

// readCloserWrapper combines an io.Reader with an io.Closer.
type readCloserWrapper struct {
	io.Reader
	c io.Closer
}

func (rcw *readCloserWrapper) Close() error {
	return rcw.c.Close()
}

type seekableReadCloser struct {
	io.ReadSeeker
}

func (s *seekableReadCloser) Close() error {
	return nil // no-op; nothing to close for in-memory data
}

func makeSeekable(r io.ReadCloser) (io.ReadSeeker, error) {
	// Read the entire content into memory.
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to buffer reader: %w", err)
	}
	return bytes.NewReader(buf), nil
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("playAudio: No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	// Convert the currentReader to a seekable stream.
	seekableReader, err := makeSeekable(currentReader)
	if err != nil {
		log.Println("Failed to make reader seekable:", err)
		return
	}

	// Wrap seekableReader in a closer to satisfy mp3 decoder
	rc := &seekableReadCloser{seekableReader}

	var audioStreamer beep.StreamSeekCloser
	var format beep.Format
	audioType, _ := detectMagicBytes(seekableReader)
	switch audioType {
	case ".mp3":
		log.Println("Using mp3 decoder")
		audioStreamer, format, err = mp3.Decode(rc)
	case ".wav":
		log.Println("Using wav decoder")
		audioStreamer, format, err = wav.Decode(rc)
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

	//ctrl := &beep.Ctrl{Streamer: loopStreamer}
	// Resample to hardcoded 44100
	resampler := beep.Resample(4, format.SampleRate, 44100, loopStreamer) // TODO: remove magic number
	volume := &effects.Volume{Streamer: resampler, Base: 0}

	log.Println("Play NOW")
	done := make(chan bool)
	currentState = Playing
	speaker.Play(beep.Seq(volume, beep.Callback(func() {
		done <- true
	})))
	currentState = Finished
}
