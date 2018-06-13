package pingproto

import (
	"bytes"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"
)

type slowReader struct {
	r    io.Reader
	tick <-chan time.Time
}

func (sr slowReader) Read(p []byte) (int, error) {
	<-sr.tick
	return sr.r.Read(p)
}

func TestPingProto(t *testing.T) {
	// TestPingProto tries sending some amount of data over a wire, slowly. In
	// the meantime, we send pings at a frequency that implies the wire should
	// contain some number of 'empty' packets - those with the delimiter length
	// being 0.
	// TestPingProto also checks that the application layer bytes are correct.

	pingTestMu.Lock()
	defer pingTestMu.Unlock()

	const (
		MiB        = 1 << 20
		readerSize = 1 * MiB
	)

	// Application are the bytes at the application layer.
	var application [readerSize]byte
	randReader := io.LimitReader(rand.New(rand.NewSource(0)), readerSize)
	_, err := io.ReadFull(randReader, application[:])
	if err != nil {
		t.Fatal(err)
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	slowApplication := slowReader{bytes.NewReader(application[:]), ticker.C}

	// slowApplication is going to take nExpectedPings * 10 * time.Millisecond. Use a ping
	// period of every 1 millisecond, and therefore should expect at least nExpectedPings pings in
	// that time.
	oldPingPeriod := pingPeriod
	pingPeriod = 1 * time.Millisecond
	defer func() { pingPeriod = oldPingPeriod }()

	// Wire contains the length delimited writes by the application layer.
	var wire bytes.Buffer
	w := NewWriter(&wire)

	const nExpectedPings = 4

	// Copy buffer size implies nExpectedPings calls to Read().
	var buf [readerSize / nExpectedPings]byte

	_, err = io.CopyBuffer(w, slowApplication, buf[:])
	if err != nil {
		t.Fatal(err)
	}

	w.Close()

	// At this point, 'wire' contains delimited application bytes and ping packets.

	var nPings int
	oldPingTestCallback := pingTestCallback
	pingTestCallback = func() { nPings++ }
	defer func() { pingTestCallback = oldPingTestCallback }()

	r := NewReader(&wire)

	applicationOut, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if nPings < nExpectedPings {
		t.Fatalf("nPings < nExpectedPings (%d < %d)",
			nPings, nExpectedPings)
	}

	if len(applicationOut) != len(application) {
		t.Fatalf("len(applicationOut) != len(application) (%d != %d)",
			len(applicationOut), len(application))
	}

	if !bytes.Equal(applicationOut, application[:]) {
		t.Fatal("applicationOut bytes not equal to applicationIn bytes")
	}
}
