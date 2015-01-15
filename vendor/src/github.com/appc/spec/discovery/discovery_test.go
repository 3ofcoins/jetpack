package discovery

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

func fakeHTTPGet(filename string, failures int) func(uri string) (*http.Response, error) {
	attempts := 0
	return func(uri string) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		var resp *http.Response

		switch {
		case attempts < failures:
			resp = &http.Response{
				Status:     "404 Not Found",
				StatusCode: http.StatusNotFound,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}
		default:
			resp = &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: f,
			}
		}

		attempts = attempts + 1
		return resp, nil
	}
}

type httpgetter func(uri string) (*http.Response, error)

func TestDiscoverEndpoints(t *testing.T) {
	tests := []struct {
		get                    httpgetter
		expectDiscoverySuccess bool
		app                    App
		expectedSig            []string
		expectedACI            []string
		expectedKeys           []string
	}{
		{
			fakeHTTPGet("myapp.html", 0),
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.sig?torrent",
				"hdfs://storage.example.com/example.com/myapp-1.0.0.sig"},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.aci?torrent",
				"hdfs://storage.example.com/example.com/myapp-1.0.0.aci"},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		{
			fakeHTTPGet("myapp.html", 1),
			true,
			App{
				Name: "example.com/myapp/foobar",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]string{"https://storage.example.com/example.com/myapp/foobar-1.0.0.sig?torrent",
				"hdfs://storage.example.com/example.com/myapp/foobar-1.0.0.sig"},
			[]string{"https://storage.example.com/example.com/myapp/foobar-1.0.0.aci?torrent",
				"hdfs://storage.example.com/example.com/myapp/foobar-1.0.0.aci"},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		{
			fakeHTTPGet("myapp.html", 20),
			false,
			App{
				Name: "example.com/myapp/foobar/bazzer",
				Labels: map[string]string{
					"version": "1.0.0",
					"os":      "linux",
					"arch":    "amd64",
				},
			},
			[]string{},
			[]string{},
			[]string{},
		},
		// Test missing label. Only one ac-discovery template should be
		// returned as the other one cannot be completely rendered due to
		// missing labels.
		{
			fakeHTTPGet("myapp2.html", 0),
			true,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"version": "1.0.0",
				},
			},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.sig"},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.aci"},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		// Test missing labels. version label should default to
		// "latest" and the first template should be rendered
		{
			fakeHTTPGet("myapp2.html", 0),
			false,
			App{
				Name:   "example.com/myapp",
				Labels: map[string]string{},
			},
			[]string{"https://storage.example.com/example.com/myapp-latest.sig"},
			[]string{"https://storage.example.com/example.com/myapp-latest.aci"},
			[]string{"https://example.com/pubkeys.gpg"},
		},
		// Test with a label called "name". It should be ignored.
		{
			fakeHTTPGet("myapp2.html", 0),
			false,
			App{
				Name: "example.com/myapp",
				Labels: map[string]string{
					"name":    "labelcalledname",
					"version": "1.0.0",
				},
			},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.sig"},
			[]string{"https://storage.example.com/example.com/myapp-1.0.0.aci"},
			[]string{"https://example.com/pubkeys.gpg"},
		},
	}

	for i, tt := range tests {
		httpGet = tt.get
		de, err := DiscoverEndpoints(tt.app, true)
		if err != nil && !tt.expectDiscoverySuccess {
			continue
		}
		if err != nil {
			t.Fatalf("#%d DiscoverEndpoints failed: %v", i, err)
		}

		if len(de.Sig) != len(tt.expectedSig) {
			t.Errorf("Sig array is wrong length want %d got %d", len(tt.expectedSig), len(de.Sig))
		} else {
			for n, _ := range de.Sig {
				if de.Sig[n] != tt.expectedSig[n] {
					t.Errorf("#%d sig[%d] mismatch: want %v got %v", i, n, tt.expectedSig[n], de.Sig[n])
				}
			}
		}

		if len(de.ACI) != len(tt.expectedACI) {
			t.Errorf("ACI array is wrong length want %d got %d", len(tt.expectedACI), len(de.ACI))
		} else {
			for n, _ := range de.ACI {
				if de.ACI[n] != tt.expectedACI[n] {
					t.Errorf("#%d sig[%d] mismatch: want %v got %v", i, n, tt.expectedACI[n], de.ACI[n])
				}
			}
		}

		if len(de.Keys) != len(tt.expectedKeys) {
			t.Errorf("Keys array is wrong length want %d got %d", len(tt.expectedKeys), len(de.Keys))
		} else {
			for n, _ := range de.Keys {
				if de.Keys[n] != tt.expectedKeys[n] {
					t.Errorf("#%d sig[%d] mismatch: want %v got %v", i, n, tt.expectedKeys[n], de.Keys[n])
				}
			}
		}
	}
}
