package main

import (
	"bytes"
	"errors"
	"fmt"
	"gioui.org/app"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
	"io"
	"log"
)

var currentReader io.ReadCloser
var currentUnit *playbackUnit

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

type playbackUnit struct {
	format    beep.Format
	streamer  beep.StreamSeeker
	ctrl      *beep.Ctrl
	resampler *beep.Resampler
	volume    *effects.Volume
	AudioType string // e.g. .mp3 or .wav
}

func (p *playbackUnit) togglePause() {
	if p.ctrl != nil {
		speaker.Lock()
		log.Println("toggled pause")
		p.ctrl.Paused = !p.ctrl.Paused
		speaker.Unlock()
	} else {
		log.Println("p.ctrl was nil!")
	}
}

func newPlaybackUnit(reader io.ReadCloser) (*playbackUnit, error) {
	var err error
	unit := &playbackUnit{}

	// Convert the currentReader to a seekable stream (read whole file into memory)
	seekableReader, err := makeSeekable(currentReader)
	if err != nil {
		log.Println("Failed to make reader seekable:", err)
		return nil, err
	}

	// Wrap seekableReader in a closer to satisfy mp3 decoder
	rc := &seekableReadCloser{seekableReader}

	audioType, _ := detectMagicBytes(seekableReader)
	switch audioType {
	case ".mp3":
		log.Println("Using mp3 decoder")
		unit.streamer, unit.format, err = mp3.Decode(rc)
	case ".wav":
		log.Println("Using wav decoder")
		unit.streamer, unit.format, err = wav.Decode(rc)
	default:
		return nil, fmt.Errorf("no decoder available for %v", audioType)
	}
	log.Println("Audio format", unit.format)

	if err != nil {
		return nil, fmt.Errorf("decoder failed for %v: %v", audioType, err)
	}

	log.Println("Build loop streamer")
	loopStreamer, err := beep.Loop2(unit.streamer, beep.LoopTimes(1))
	if err != nil {
		log.Println("loop2 err:", err)
		return nil, err
	}

	unit.ctrl = &beep.Ctrl{Streamer: loopStreamer}
	// Resample to hardcoded 44100
	resampler := beep.Resample(4, unit.format.SampleRate, 44100, unit.ctrl) // TODO: remove magic number
	unit.volume = &effects.Volume{Streamer: resampler, Base: 0}
	return unit, nil
}

// playAudio now uses a TeeReader to split the stream.
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("playAudio: No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	playbackUnit, err := newPlaybackUnit(currentReader)
	if err != nil {
		log.Println("Couldn't create playback unit:", err)
	}
	log.Println("Play NOW")
	done := make(chan bool)
	currentUnit = playbackUnit
	currentState = Playing
	speaker.Play(beep.Seq(playbackUnit.volume, beep.Callback(func() {
		done <- true
		log.Println("Audio DONE")
		currentState = Finished
	})))
}
