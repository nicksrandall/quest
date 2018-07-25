package quest

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/nicksrandall/quest/questmultipart"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// Request is the HTTP request to be sent
type Request struct {
	*url.URL
	transport *http.Transport
	method    string
	data      *bytes.Buffer
	headers   map[string]string
	err       error
	ctx       context.Context
}

// New creates a new request with given http method and path (uri)
func New(method, path string) *Request {
	u, err := url.Parse(path)
	if err != nil {
		return &Request{err: fmt.Errorf("error parsing url %q: %v", path, err)}
	}

	return &Request{
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
func Get(path string) *Request {
	return New(http.MethodGet, path)
}

// Post creates a new http "POST" request for path (uri)
func Post(path string) *Request {
	return New(http.MethodPost, path)
}

// Put creates a new http "Put" request for path (uri)
func Put(path string) *Request {
	return New(http.MethodPut, path)
}

// Delete creates a new http "Delete" request for path (uri)
func Delete(path string) *Request {
	return New(http.MethodDelete, path)
}

// WithContext sets up a context for this request
func (r *Request) WithContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// Header sets a header on request with given key and value
func (r *Request) Header(key, value string) *Request {
	if r.err != nil {
		return r
	}
	r.headers[key] = value
	return r
}

// BasicAuth sets a header on request with given key and value
func (r *Request) BasicAuth(username, password string) *Request {
	if r.err != nil {
		return r
	}
	auth := username + ":" + password
	r.headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	return r
}

// QueryParam adds a query param to the url
func (r *Request) QueryParam(key, value string) *Request {
	if r.err != nil {
		return r
	}
	q := r.URL.Query()
	q.Add(key, value)
	r.URL.RawQuery = q.Encode()
	return r
}

// Param replaces url param (denoted with `:key`) with given value
func (r *Request) Param(key, value string) *Request {
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
func (r *Request) Body(value *bytes.Buffer) *Request {
	if r.err != nil {
		return r
	}
	r.data = value
	return r
}

// JSONBody sets the given value as a JSON encoded string as the body of the request
func (r *Request) JSONBody(value interface{}) *Request {
	if r.err != nil {
		return r
	}
	b, err := jsoniter.Marshal(value)
	if err != nil {
		r.err = handleRequestError(err, r)
		return r
	}
	r.Header("Content-Type", "application/json")
	return r.Body(bytes.NewBuffer(b))
}

// MultipartBody will set a multipart form as the body of the request
func (r *Request) MultipartBody(form *questmultipart.Form) *Request {
	if r.err != nil {
		return r
	}
	r.Header("Content-Type", form.Writer.FormDataContentType())
	r.err = form.Err
	return r.Body(form.Buffer)
}

// WithTransport sets the transport for the http client
func (r *Request) WithTransport(transport *http.Transport) *Request {
	if r.err != nil {
		return r
	}
	r.transport = transport
	return r
}

// Send sends the request and returns the response
func (r *Request) Send() *Response {
	if r.err != nil {
		return &Response{
			Response: &http.Response{},
			req:      r,
		}
	}

	client := &http.Client{}
	if r.transport != nil {
		client.Transport = r.transport
	}

	req, err := http.NewRequest(r.method, r.URL.String(), r.data)
	if err != nil {
		r.err = handleRequestError(err, r)
		return &Response{
			Response: &http.Response{},
			req:      r,
		}
	}

	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	if r.ctx != nil {
		req = req.WithContext(r.ctx)
		span, _ := opentracing.StartSpanFromContext(r.ctx, "Quest: request")
		span.SetTag("http.method", r.method)
		span.SetTag("http.host", r.URL.Host)
		span.SetTag("http.path", r.URL.Path)
		ext.HTTPUrl.Set(
			span,
			fmt.Sprintf("%s://%s%s", r.URL.Scheme, r.URL.Host, r.URL.Path),
		)

		opentracing.GlobalTracer().Inject(
			span.Context(),
			opentracing.HTTPHeaders,
			opentracing.HTTPHeadersCarrier(req.Header),
		)

		defer span.Finish()
	}

	resp, err := client.Do(req)
	if err != nil {
		r.err = handleRequestError(err, r)
		return &Response{
			Response: resp,
			req:      r,
		}
	}

	return &Response{
		Response: resp,
		req:      r,
	}
}
