package pingproto

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPContentEncodingNegotiation(t *testing.T) {
	s := httptest.NewServer(
		http.HandlerFunc(func(wWire http.ResponseWriter, r *http.Request) {
			wApp := HTTPTryContentEncoding(wWire, r)
			defer wApp.Close()

			// Has the effect of writing a ping if the upgrade has happened,
			// otherwise NO-OP.
			wApp.Write(nil)

			io.WriteString(wApp, "Hello, world\n")
		}))
	defer s.Close()

	doRequest := func(
		t *testing.T,
		c *http.Client,
		shouldBeDecoder bool,
	) {
		pingTestMu.Lock()
		defer pingTestMu.Unlock()

		var nPings int
		oldPingTestCallback := pingTestCallback
		pingTestCallback = func() { nPings++ }
		defer func() { pingTestCallback = oldPingTestCallback }()

		resp, err := c.Get(s.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if _, ok := resp.Body.(isDecoder); ok != shouldBeDecoder {
			if shouldBeDecoder {
				t.Logf("resp.Body should be a decoder: %T", resp.Body)
			} else {
				t.Logf("resp.Body should not be a decoder: %T", resp.Body)
			}
		}

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal([]byte("Hello, world\n"), content) {
			t.Fatalf("Unexpected content: %q", content)
		}

		if shouldBeDecoder {
			if nPings < 1 { // expect at least one ping.
				t.Logf("Missing pings: nPings < 1 : (%d < 1)", nPings)
			}
		} else {
			if nPings != 0 {
				t.Logf("Unexpected pings: nPings != 0 : (%d != 0)", nPings)
			}
		}
	}

	t.Run("withPingProtoClient", func(t *testing.T) {
		c := NewHTTPClient(nil) // pingproto
		doRequest(t, c, true)
	})
	t.Run("withPlainClient", func(t *testing.T) {
		c := http.DefaultClient // not pingproto
		doRequest(t, c, false)
	})
}
