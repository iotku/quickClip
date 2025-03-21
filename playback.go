package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"gioui.org/app"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

var currentReader io.ReadCloser
var currentUnit *playbackUnit

const bufferSize = 44100 * 2 * 2 // 1 second of STEREO audio at 44.1kHz
var audioRingBuffer = make([]byte, bufferSize)
var ringWritePos = 0
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
	case len(header) >= 4 && string(header[:4]) == "fLaC":
		return ".flac"
	default:
		log.Println("Could not determine audio type by magic bytes")
		return ""
	}
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
	done      chan bool
	AudioType string // e.g. ".wav", ".flac", or ".mp3"
}

func (p *playbackUnit) forward() (err error) {
	if p == nil {
		return
	}
	speaker.Lock()
	newPos := p.streamer.Position()
	newPos += p.format.SampleRate.N(time.Second * 5)
	// Clamp the position to be within the stream
	newPos = max(newPos, 0)
	newPos = min(newPos, p.streamer.Len()-1)

	if err = p.streamer.Seek(newPos); err != nil {
		log.Println(err)
	}
	speaker.Unlock()
	return err
}

func (p *playbackUnit) back() (err error) {
	if p == nil {
		return
	}
	speaker.Lock()
	newPos := p.streamer.Position()
	newPos -= p.format.SampleRate.N(2500 * time.Millisecond)
	// Clamp the position to be within the stream
	newPos = max(newPos, 0)
	newPos = min(newPos, p.streamer.Len()-1)

	if err = p.streamer.Seek(newPos); err != nil {
		log.Println(err)
	}
	speaker.Unlock()
	return err
}

func (p *playbackUnit) setPaused(state bool) {
	if p == nil {
		return
	}

	speaker.Lock()
	log.Println("Setting paused to:", state)
	p.ctrl.Paused = state
	speaker.Unlock()

}

// Set volume level of the playbackUnit from 0.0 (0%) to 1.0 (100%)
func (p *playbackUnit) setVolume(level float32) {
	if p == nil {
		return
	}
	playbackVolume = float64(level)
	if level == 0.0 {
		p.volume.Silent = true
		return
	}

	percentage := level * 100
	if percentage < 0 {
		percentage = 0
	} else if percentage > 100 {
		percentage = 100
	}

	dB := 60 * (percentage/100 - 1)
	p.volume.Base = 2
	p.volume.Volume = float64(dB / 10)
	p.volume.Silent = false
}

// return the percentage of playback progress as a float32 (e.g. for progressbar updates)
func (p *playbackUnit) getProgressFloat() float32 {
	if p == nil {
		return 0.0
	}
	totalSamples := p.streamer.Len()
	if totalSamples > 0 {
		return float32(p.streamer.Position()) / float32(totalSamples)
	}
	return 0.0
}

func newPlaybackUnit(reader io.ReadCloser) (*playbackUnit, error) {
	var err error
	unit := &playbackUnit{done: make(chan bool)}

	// Convert the currentReader to a seekable stream (read whole file into memory)
	seekableReader, err := makeSeekable(reader)
	if err != nil {
		log.Println("Failed to make reader seekable:", err)
		return nil, err
	}

	// Wrap seekableReader in a closer to satisfy mp3 decoder
	rc := &seekableReadCloser{seekableReader}

	audioType, err := detectMagicBytes(seekableReader)
	if err != nil {
		return nil, err
	}
	switch audioType {
	case ".mp3":
		log.Println("Using mp3 decoder")
		unit.streamer, unit.format, err = mp3.Decode(rc)
	case ".wav":
		log.Println("Using wav decoder")
		unit.streamer, unit.format, err = wav.Decode(rc)
	case ".flac":
		log.Println("Using flac decoder")
		unit.streamer, unit.format, err = flac.Decode(rc)
	default:
		return nil, fmt.Errorf("no decoder available for %v", audioType)
	}
	log.Println("Audio format", unit.format)

	if err != nil {
		return nil, fmt.Errorf("decoder failed for %v: %v", audioType, err)
	}

	loopStreamer, err := beep.Loop2(unit.streamer, beep.LoopTimes(0))
	if err != nil {
		log.Println("loop2 err:", err)
		return nil, err
	}

	unit.ctrl = &beep.Ctrl{Streamer: loopStreamer}
	// Resample to hardcoded 44100
	resampler := beep.Resample(4, unit.format.SampleRate, 44100, unit.ctrl) // TODO: remove magic number
	tap := &TapStreamer{s: resampler}
	unit.volume = &effects.Volume{Streamer: tap}
	unit.setVolume(float32(playbackVolume)) // set default volume
	return unit, nil
}

// playAudio plays the current (last) reader provided by the file explorer
func playAudio(w *app.Window) {
	if currentReader == nil {
		log.Println("playAudio: No audio reader")
		return
	} else if currentState == Playing {
		return
	}

	if currentUnit != nil {
		speaker.Clear()
	}
	playbackUnit, err := newPlaybackUnit(currentReader)
	if err != nil {
		log.Println("Couldn't create playback unit:", err)
	}
	log.Println("Play NOW")
	currentUnit = playbackUnit
	if playbackUnit == nil || playbackUnit.volume == nil {
		log.Println("Playback unit streamer not available")
		return
	}
	currentState = Playing

	speaker.Play(beep.Seq(playbackUnit.volume, beep.Callback(func() {
		playbackUnit.done <- true
	})))

	ticker := time.NewTicker(time.Millisecond * 16) // ~60 FPS
	defer ticker.Stop()

	progressTicker := time.NewTicker(time.Second)
	defer progressTicker.Stop()

	for {
		select {

		// Update playback progressbar
		case <-progressTicker.C:
			updateProgressBar(playbackUnit)

		case <-ticker.C: // Force redraw at ticker interval for smooth waveform
			w.Invalidate()

		case <-playbackUnit.done:
			log.Println("Audio DONE")
			resetVisualization()
			resetProgressBar()
			currentState = Finished
			w.Invalidate()
			return
		}
	}
}
