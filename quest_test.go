package quest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const TestString = "Hello, world!"
const Header = "X-Some-Header"

func Auth(token string) (string, string) {
	return Header, token
}

func TestQuest(t *testing.T) {
	var body string
	var header string
	var token = "some-fake-token"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(Header) != token {
			t.Error("Header was not set on request")
		}
		w.Header().Set(Header, token)
		isBad := r.URL.Query().Get("bad")
		if isBad != "" {
			w.WriteHeader(400)
		}
		fmt.Fprint(w, TestString)
	}))
	defer ts.Close()

	err := Get(ts.URL).
		Header(Auth(token)).
		Send().
		ExpectSuccess().
		GetHeader(Header, &header).
		GetBody(&body).
		Done()

	if err != nil {
		t.Error(err.Error())
	}

	if body != TestString {
		t.Errorf("Response body did not match: %q, %q", body, TestString)
	}

	if header != token {
		t.Errorf("Response header was not set: %q, %q", header, token)
	}

	// test never closing the response.body
	err = Get(ts.URL + "?bad=true").
		Header(Auth(token)).
		Send().
		ExpectStatusCode(400).
		Done()

	if err != nil {
		t.Error(err.Error())
	}
}

func Example() {
	var body interface{}
	err := Get("path/to/some/resource").
		Header("X-Some-Header", "value").
		Send().
		ExpectSuccess().
		GetJSON(&body).
		Done()

	if err != nil {
		// handle error
	}

	// do something with body
}
