package quest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/nicksrandall/quest/questmultipart"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

type request struct {
	*url.URL
	method  string
	data    *bytes.Buffer
	headers map[string]string
	err     error
	span    opentracing.Span
}

type response struct {
	*http.Response
	req *request
}

type next struct {
	err error
}

type requestError struct {
	message string
	Request *request
}

type responseError struct {
	message  string
	Request  *request
	Response *response
}

func (e requestError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s", e.message, e.Request.format())
}

func (e responseError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s\n\nResponse Info:\n %s", e.message, e.Request.format(), e.Response.format())
}

// New creates a new request with given http method and path (uri)
func New(method, path string) *request {
	u, err := url.Parse(path)
	if err != nil {
		return &request{err: fmt.Errorf("error parsing url %q: %v", path, err)}
	}

	return &request{
		URL:    u,
		method: method,
		headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "quest/v1",
		},
		data: &bytes.Buffer{},
	}
}

// Get creates a new http "GET" request for path (uri)
func Get(path string) *request {
	return New(http.MethodGet, path)
}

// Post creates a new http "POST" request for path (uri)
func Post(path string) *request {
	return New(http.MethodPost, path)
}

// Put creates a new http "Put" request for path (uri)
func Put(path string) *request {
	return New(http.MethodPut, path)
}

// Delete creates a new http "Delete" request for path (uri)
func Delete(path string) *request {
	return New(http.MethodDelete, path)
}

// New creates a new request with given http method and path (uri) and is
// used when chaining requests together
func (n *next) New(method, path string) *request {
	req := New(method, path)
	if req.err == nil {
		req.err = n.err
	}
	return req
}

// Get creates a new http "GET" request for path (uri) and is used when chaining requests together
func (n *next) Get(path string) *request {
	return n.New(http.MethodGet, path)
}

// Post creates a new http "POST" request for path (uri) and is used when chaining requests together
func (n *next) Post(path string) *request {
	return n.New(http.MethodPost, path)
}

// Put creates a new http "Put" request for path (uri) and is used when chaining requests together
func (n *next) Put(path string) *request {
	return n.New(http.MethodPut, path)
}

// Delete creates a new http "Delete" request for path (uri) and is used when chaining requests together
func (n *next) Delete(path string) *request {
	return n.New(http.MethodDelete, path)
}

// Span creates an open tracing span for request
func (r *request) StartSpan(ctx context.Context) *request {
	r.span, _ = opentracing.StartSpanFromContext(ctx, "Quest: request")
	return r
}

// Header sets a header on request with given key and value
func (r *request) Header(key, value string) *request {
	if r.err != nil {
		return r
	}
	r.headers[key] = value
	return r
}

// QueryParam adds a query param to the url
func (r *request) QueryParam(key, value string) *request {
	if r.err != nil {
		return r
	}
	q := r.URL.Query()
	q.Add(key, value)
	r.URL.RawQuery = q.Encode()
	return r
}

// Param replaces url param (denoted with `:key`) with given value
func (r *request) Param(key, value string) *request {
	if r.err != nil {
		return r
	}
	path := strings.Replace(r.URL.String(), ":"+key, value, 1)
	url, err := url.Parse(path)
	if err != nil {
		r.err = handleRequestError(err, r)
		return r
	}
	r.URL = url
	return r
}

// Body sets the body for the request
func (r *request) Body(value *bytes.Buffer) *request {
	if r.err != nil {
		return r
	}
	r.data = value
	return r
}

// JSONBody sets the given value as a JSON encoded string as the body of the request
func (r *request) JSONBody(value interface{}) *request {
	if r.err != nil {
		return r
	}
	b, err := json.Marshal(value)
	if err != nil {
		r.err = handleRequestError(err, r)
		return r
	}
	r.Header("Content-Type", "application/json")
	return r.Body(bytes.NewBuffer(b))
}

// MultipartBody will set a multipart form as the body of the request
func (r *request) MultipartBody(form *questmultipart.Form) *request {
	if r.err != nil {
		return r
	}
	r.Header("Content-Type", form.Writer.FormDataContentType())
	r.err = form.Err
	return r.Body(form.Buffer)
}

// Send sends the request and returns the response
func (r *request) Send() *response {
	if r.err != nil {
		return &response{
			Response: &http.Response{},
			req:      r,
		}
	}

	client := &http.Client{}

	if r.span != nil {
		r.span.SetTag("http.method", r.method)
		r.span.SetTag("http.host", r.URL.Host)
		r.span.SetTag("http.path", r.URL.Path)
		ext.HTTPUrl.Set(
			r.span,
			fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path),
		)
		defer r.span.Finish()
	}

	req, err := http.NewRequest(r.method, r.URL.String(), r.data)
	if err != nil {
		r.err = handleRequestError(err, r)
		return &response{
			Response: &http.Response{},
			req:      r,
		}
	}

	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	if r.span != nil {
		opentracing.GlobalTracer().Inject(
			r.span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(req.Header),
		)
	}

	resp, err := client.Do(req)
	if err != nil {
		r.err = handleRequestError(err, r)
		return &response{
			Response: resp,
			req:      r,
		}
	}

	return &response{
		Response: resp,
		req:      r,
	}
}

// ExpectSuccess will error if StatusCode is not in 200 range
func (r *response) ExpectSuccess() *response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.StatusCode; actual < 200 || actual >= 300 {
		err := errors.New(fmt.Sprintf("Invalid StatusCode. Expected to be in 200 range, got '%d'", actual))
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectStatusCode will error if StatusCode is not specified code
func (r *response) ExpectStatusCode(code int) *response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.StatusCode; actual != code {
		err := errors.New(fmt.Sprintf("Invalid StatusCode. Expected to be '%d', got '%d'", code, actual))
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectHeader will error if given header is not set with given value
func (r *response) ExpectHeader(key, value string) *response {
	if r.req.err != nil {
		return r
	}
	if actual := r.Response.Header.Get(key); !strings.Contains(actual, value) {
		err := errors.New(fmt.Sprintf("Invalid Header. Expected %q header to be %q, got %q", key, value, actual))
		r.req.err = handleResponseError(err, r.req, r)
		return r
	}
	return r
}

// ExpectType will error if header "Content-Type" is not specified value
func (r *response) ExpectType(value string) *response {
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
func (r *response) GetHeader(key string, into *string) *response {
	if r.req.err != nil {
		return r
	}
	*into = r.Response.Header.Get(key)
	return r
}

// PrintJSON will print response as json, can be use for debugging purposes
func (r *response) PrintJSON() *response {
	if r.req.err != nil {
		return r
	}
	buffer, _ := ioutil.ReadAll(r.Response.Body)
	dst := bytes.Buffer{}
	json.Indent(&dst, buffer, "*", "\t")
	fmt.Printf("Response JSON:")
	dst.WriteTo(os.Stdout)
	fmt.Println("")
	r.Response.Body = ioutil.NopCloser(bytes.NewBuffer(buffer))
	return r
}

// GetBody stores the response body into into param
func (r *response) GetBody(into *string) *response {
	if r.req.err != nil {
		return r
	}
	defer r.Response.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Response.Body)
	*into = buf.String()
	return r
}

// GetJSON decodes and stores the response body
func (r *response) GetJSON(into interface{}) *response {
	if r.req.err != nil {
		return r
	}
	defer r.Response.Body.Close()
	dec := json.NewDecoder(r.Response.Body)
	err := dec.Decode(into)
	if err != nil {
		r.req.err = handleResponseError(err, r.req, r)
	}
	return r
}

// Next allows a new request to be chained onto this request, assuming the first request
// did not fail
func (r *response) Next() *next {
	return &next{r.req.err}
}

// Done will return the first error that occured durring the request's life-cycle
//
// It is important to note that if any method errors, all subsequest methods will short
// circut and not be execuited
func (r *response) Done() error {
	return r.req.err
}

// MarshalJSON implements `json.Marshaler` interface
func (r *request) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(requestJSON{
		r.URL,
		r.method,
		string(r.data.Bytes()),
		r.headers,
	}, "", "  ")
}

// UnmarshalJSON implements `json.Unmarshaler` interface
func (r *request) UnmarshalJSON(b []byte) error {
	temp := &requestJSON{}
	if err := json.Unmarshal(b, &temp); err != nil {
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

// MarshalJSON implements `json.Marshaler` interface
func (r *response) MarshalJSON() ([]byte, error) {
	body, _ := ioutil.ReadAll(r.Response.Body)
	return json.MarshalIndent(responseJSON{
		r.Response.StatusCode,
		r.Response.Header,
		string(body),
		r.Response.ContentLength,
	}, "", "  ")
}

// UnmarshalJSON implements `json.Unmarshaler` interface
func (r *response) UnmarshalJSON(b []byte) error {
	// not implemented
	return nil
}

func (r *request) format() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func (r *response) format() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func handleRequestError(err error, req *request) *requestError {
	return &requestError{
		message: err.Error(),
		Request: req,
	}
}

func handleResponseError(err error, req *request, resp *response) *responseError {
	return &responseError{
		message:  err.Error(),
		Request:  req,
		Response: resp,
	}
}
