package quest

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/json-iterator/go"
)

// Response is the HTTP response
type Response struct {
	*http.Response
	req *Request
}

// Proxy copies the body of the response to a given writer
func (r *Response) Proxy(w io.Writer) *Response {
	if r.req.err != nil {
		return r
	}
	_, err := io.Copy(w, r.Response.Body)
	if err != nil {
		r.req.err = handleResponseError(err, r.req, r)
	}
	return r
}

// ExpectSuccess will error if StatusCode is not in 200 range
func (r *Response) ExpectSuccess() *Response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.StatusCode; actual < 200 || actual >= 300 {
		err := fmt.Errorf("Invalid StatusCode. Expected to be in 200 range, got '%d'", actual)
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectStatusCode will error if StatusCode is not specified code
func (r *Response) ExpectStatusCode(code int) *Response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.StatusCode; actual != code {
		err := fmt.Errorf("Invalid StatusCode. Expected to be '%d', got '%d'", code, actual)
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectHeader will error if given header is not set with given value
func (r *Response) ExpectHeader(key, value string) *Response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.Header.Get(key); !strings.Contains(actual, value) {
		err := fmt.Errorf("Invalid Header. Expected %q header to be %q, got %q", key, value, actual)
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectType will error if header "Content-Type" is not specified value
func (r *Response) ExpectType(value string) *Response {
	if r.req.err != nil {
		return r
	}

	// Types is a map of MIME type aliases
	var types = map[string]string{
		"html":       "text/html",
		"json":       "application/json",
		"xml":        "application/xml",
		"text":       "text/plain",
		"urlencoded": "application/x-www-form-urlencoded",
		"form":       "application/x-www-form-urlencoded",
		"form-data":  "application/x-www-form-urlencoded",
	}

	var typeValue string
	if v, ok := types[value]; ok {
		typeValue = v
	} else {
		typeValue = value
	}

	return r.ExpectHeader("Content-Type", typeValue)
}

// GetHeader stores header value with key into into paramiter
func (r *Response) GetHeader(key string, into *string) *Response {
	if r.req.err != nil {
		return r
	}
	*into = r.Response.Header.Get(key)
	return r
}

// GetBody stores the response body into into param
func (r *Response) GetBody(into *string) *Response {
	if r.req.err != nil {
		return r
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Response.Body)
	*into = buf.String()
	return r
}

// GetJSON decodes and stores the response body
func (r *Response) GetJSON(into interface{}) *Response {
	if r.req.err != nil {
		return r
	}
	dec := jsoniter.NewDecoder(r.Response.Body)
	err := dec.Decode(into)
	if err != nil {
		r.req.err = handleResponseError(err, r.req, r)
	}
	return r
}

// Next allows a new request to be chained onto this request, assuming the first request
// did not fail
func (r *Response) Next() *Next {
	return &Next{r.req.err}
}

// Done will return the first error that occured durring the request's life-cycle
//
// It is important to note that if any method errors, all subsequest methods will short
// circut and not be execuited
func (r *Response) Done() error {
	r.Body.Close()
	return r.req.err
}

// MarshalJSON implements `jsoniter.Marshaler` interface
func (r *Request) MarshalJSON() ([]byte, error) {
	return jsoniter.MarshalIndent(requestJSON{
		r.URL,
		r.method,
		string(r.data.Bytes()),
		r.headers,
	}, "", "  ")
}

// UnmarshalJSON implements `jsoniter.Unmarshaler` interface
func (r *Request) UnmarshalJSON(b []byte) error {
	temp := &requestJSON{}
	if err := jsoniter.Unmarshal(b, &temp); err != nil {
		return err
	}

	r.URL = temp.URL
	r.method = temp.Method
	r.data = bytes.NewBuffer([]byte(temp.Data))
	r.headers = temp.Headers

	return nil
}

type requestJSON struct {
	*url.URL
	Method  string
	Data    string
	Headers map[string]string
}

type responseJSON struct {
	StatusCode    int
	Header        http.Header
	Body          string
	ContentLength int64
}

// MarshalJSON implements `jsoniter.Marshaler` interface
func (r *Response) MarshalJSON() ([]byte, error) {
	defer r.Response.Body.Close()
	body, _ := ioutil.ReadAll(r.Response.Body)
	return jsoniter.MarshalIndent(responseJSON{
		r.Response.StatusCode,
		r.Response.Header,
		string(body),
		r.Response.ContentLength,
	}, "", "  ")
}

// UnmarshalJSON implements `jsoniter.Unmarshaler` interface
func (r *Response) UnmarshalJSON(b []byte) error {
	// not implemented
	return nil
}

func (r *Request) format() string {
	b, _ := jsoniter.MarshalIndent(r, "", "  ")
	return string(b)
}

func (r *Response) format() string {
	b, _ := jsoniter.MarshalIndent(r, "", "  ")
	return string(b)
}
