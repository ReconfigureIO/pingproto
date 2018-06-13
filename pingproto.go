// Package pingproto implements a protocol which transparently inserts packets
// periodically with the intention of keeping a long-lived stream alive.
package pingproto

import (
	"encoding/binary"
	"io"
	"log"
	"sync"
	"time"
)

// pingPeriod controls the frequency of pings. It is a global (and non-const) so
// that it can be modified in tests.
var (
	pingTestMu       sync.Mutex
	pingPeriod       = 10 * time.Second
	pingTestCallback = func() {}
)

// NewReader wraps a reader, stripping out the pingproto layer.
// The caller must close the returned reader.
func NewReader(rIn io.Reader) io.ReadCloser {
	rOut, w := io.Pipe()
	go (&decoder{r: rIn, w: w}).process()
	return rOut
}

type decoder struct {
	r io.Reader
	w *io.PipeWriter
}

func (d *decoder) process() {
	var (
		length int
		err    error
		buf    [1 << 16]byte
	)

	defer func() {
		// Note: err may be nil, and that's equivalent to Close().
		err2 := d.w.CloseWithError(err)
		if err2 != nil {
			log.Panicf("pingproto.NewReader: w.CloseWithError unexpected error: %v", err)
		}
	}()

	for {
		length, err = lenRead(d.r)
		if err != nil {
			break // Handled in defer.
		}
		if length == 0 {
			pingTestCallback()
			continue // Drop packet from proto.
		}
		_, err = io.CopyBuffer(d.w, io.LimitReader(d.r, int64(length)), buf[:])
		if err != nil {
			break // Handled in defer.
		}
	}
}

func lenRead(r io.Reader) (int, error) {
	var buf [4]byte
	_, err := io.ReadFull(r, buf[:])
	length := binary.LittleEndian.Uint32(buf[:])
	return int(length), err
}

// NewWriter wraps a writer, returning a new writer which can be written to.
// Under the hood, proto.NewWriter writes 4-byte ping packets onto the wire at a
// frequency of 10 seconds. The caller must close the returned writer, or a
// goroutine will be leaked.
func NewWriter(w io.Writer) io.WriteCloser {
	enc := &encoder{
		mu:     new(sync.Mutex),
		w:      w,
		closed: make(chan struct{}),
		done:   make(chan struct{}),
	}
	started := make(chan struct{})
	go enc.process(started)
	<-started
	return enc
}

type encoder struct {
	mu *sync.Mutex
	w  io.Writer
	// closed unblocks when Close() is called. done unblocks when process() will
	// not write any more ping packets in the future.
	closed, done chan struct{}
}

// Write puts p on the underlying writer with additional length delimiting. If
// len(p) == 0, write will simply write a 0 length deliminter, which is useful
// as a ping.
func (enc encoder) Write(p []byte) (int, error) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	err := lenWrite(enc.w, len(p))
	if err != nil {
		return 0, err
	}
	n, err := enc.w.Write(p)
	return n, err
}

func (enc encoder) Close() error {
	close(enc.closed)
	<-enc.done
	return nil
}

func (enc encoder) process(started chan<- struct{}) {
	defer close(enc.done)

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	close(started) // Signal to NewWriter to unblock.

	for {
		select {
		case <-ticker.C:
			// Write 0 bytes to the stream, which is the ping packet. We can
			// safely ignore any error here; errors are 'sticky', so the next
			// write will fail too.
			_, _ = enc.Write(nil)

		case <-enc.closed:
			return
		}
	}
}

func lenWrite(w io.Writer, length int) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(length))
	_, err := w.Write(buf[:])
	return err
}
