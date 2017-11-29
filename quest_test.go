package quest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var ts *httptest.Server

const TestString = "Hello, world!"

type SpecialClient struct {
	ClientInterface
}

func (c *SpecialClient) New(method string, path string) RequestInterface {
	req := c.ClientInterface.New(method, path)
	return &SpecialRequest{req}
}

type SpecialRequest struct {
	RequestInterface
}

// Extend the api to add and `Auth` method
func (r *SpecialRequest) Auth(key string) RequestInterface {
	// some fake auth logic
	r.Header("X-Some-Authentication", key)
	return r
}

type SpecialResponse struct {
	ResponseInterface
}

// override the `Next` method
func (r *SpecialResponse) Next() NextInterface {
	return &Next{&SpecialClient{}, r.GetRequest().GetError()}
}

func TestQuestEmbed(t *testing.T) {
	var body string
	client := &SpecialClient{&Client{}}
	err := client.
		New("GET", ts.URL).
		Send().
		ExpectSuccess().
		GetBody(&body).
		Done()

	if err != nil {
		t.Error(err.Error())
	}

	if body != TestString {
		t.Errorf("Response body did not match: %q, %q", body, TestString)
	}
}

func TestMain(m *testing.M) {
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, TestString)
	}))
	defer ts.Close()
	os.Exit(m.Run())
}
