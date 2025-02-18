package main

// chunkWriter implements io.Writer and sends written chunks over a channel.
type chunkWriter struct {
	ch chan<- []byte
}

func newChunkWriter(ch chan<- []byte) *chunkWriter {
	return &chunkWriter{ch: ch}
}

func (cw *chunkWriter) Write(p []byte) (n int, err error) {
	// Copy p into a new slice so it won't be modified later.
	buf := make([]byte, len(p))
	copy(buf, p)
	// Non-blocking send.
	select {
	case cw.ch <- buf:
	default:
	}
	return len(p), nil
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}
