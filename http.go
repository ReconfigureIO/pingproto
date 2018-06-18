package pingproto

import (
	"io"
	"net/http"
)

// NewHTTPClient constructs a HTTP client which understands the pingproto and
// sends an Accept-Encoding: pingproto/1.0 header.
// If wrapee is nil, defaults to http.DefaultClient.
func NewHTTPClient(wrapee *http.Client) *http.Client {
	if wrapee == nil {
		wrapee = http.DefaultClient
	}
	wrapeeCopy := *wrapee
	if wrapeeCopy.Transport == nil {
		wrapeeCopy.Transport = http.DefaultTransport
	}
	wrapeeCopy.Transport = HTTPRoundTripper{wrapeeCopy.Transport}
	return &wrapeeCopy
}

// HTTPRoundTripper does content negotiation through the Content-Εncoding
// header, and transparently unwraps pingproto requests.
type HTTPRoundTripper struct{ http.RoundTripper }

const httpEncodingName = "pingproto/1.0"

// RoundTrip implements the http.RoundTripper interface.
func (r HTTPRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Accept-Encoding", httpEncodingName)
	resp, err := r.RoundTripper.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if te := resp.Header.Get("Content-Εncoding"); te != httpEncodingName {
		return resp, err // Not a transfer encoding we recognize.
	}

	// Remove header, since the encoding is transparent to the client.
	resp.Header.Del("Content-Εncoding")

	rApp := NewReader(resp.Body) // unwrap pingproto

	resp.Body = struct {
		io.Reader
		io.Closer
		isDecoder // embedded here so that tests can tell.
	}{
		Reader: rApp,
		Closer: composeClosers(
			rApp,
			io.Closer(resp.Body),
		),
	}

	return resp, err
}

// HTTPTryContentEncoding determines if the client supports pingproto, and if
// so, upgrades the connection. It's important to close the returned WriteCloser.
func HTTPTryContentEncoding(
	w http.ResponseWriter, r *http.Request,
) (
	wApplication io.WriteCloser,
	upgraded bool,
) {
	if te := r.Header.Get("Accept-Encoding"); te != httpEncodingName {
		// No upgrade.
		return struct {
			io.Writer
			nopCloser
		}{
			Writer: w,
		}, false
	}

	w.Header().Set("Content-Εncoding", httpEncodingName)
	return NewWriter(w), true
}

// isDecoder enables the tests to tell that a response has been unwrapped.
type isDecoder interface{ private() }

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

func composeClosers(closers ...io.Closer) composeClosersImpl {
	return composeClosersImpl{closers}
}

type composeClosersImpl struct{ closers []io.Closer }

func (c composeClosersImpl) Close() error {
	var err error
	for _, closer := range c.closers {
		err2 := closer.Close()
		if err == nil { // first error is sticky.
			err = err2
		}
	}
	return err
}
