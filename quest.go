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

type Request struct {
	*url.URL
	method  string
	data    *bytes.Buffer
	headers map[string]string
	err     error
	span    opentracing.Span
}

type Response struct {
	*http.Response
	req *Request
}

type Next struct {
	ClientInterface
	err error
}

type RequestError struct {
	message string
	Request *Request
}

type ResponseError struct {
	message  string
	Request  *Request
	Response *Response
}

func (e RequestError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s", e.message, e.Request.debug())
}

func (e ResponseError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s\n\nResponse Info:\n %s", e.message, e.Request.debug(), e.Response.debug())
}

type RequestInterface interface {
	GetError() error
	SetError(error) RequestInterface
	SetContext(context.Context) RequestInterface
	Header(string, string) RequestInterface
	QueryParam(string, string) RequestInterface
	Param(string, string) RequestInterface
	Body(*bytes.Buffer) RequestInterface
	JSONBody(interface{}) RequestInterface
	MultipartBody(*questmultipart.Form) RequestInterface
	Send() ResponseInterface
}

type ResponseInterface interface {
	GetRequest() RequestInterface
	ExpectSuccess() ResponseInterface
	ExpectStatusCode(int) ResponseInterface
	ExpectHeader(string, string) ResponseInterface
	ExpectType(string) ResponseInterface
	GetHeader(string, *string) ResponseInterface
	PrintJSON() ResponseInterface
	GetBody(*string) ResponseInterface
	GetJSON(interface{}) ResponseInterface
	Next() NextInterface
	Done() error
}

type ClientInterface interface {
	New(string, string) RequestInterface
	Get(string) RequestInterface
	Post(string) RequestInterface
	Put(string) RequestInterface
	Delete(string) RequestInterface
}

type NextInterface interface {
	New(string, string) RequestInterface
	Get(string) RequestInterface
	Post(string) RequestInterface
	Put(string) RequestInterface
	Delete(string) RequestInterface
}

type Client struct{}

func (_ *Client) New(method, path string) RequestInterface {
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

func (c *Client) Get(path string) RequestInterface {
	return c.New(http.MethodGet, path)
}

func (c *Client) Post(path string) RequestInterface {
	return c.New(http.MethodPost, path)
}

func (c *Client) Put(path string) RequestInterface {
	return c.New(http.MethodPut, path)
}

func (c *Client) Delete(path string) RequestInterface {
	return c.New(http.MethodDelete, path)
}

func (n *Next) New(method, path string) RequestInterface {
	req := n.ClientInterface.New(method, path)
	if req.GetError() == nil {
		req.SetError(n.err)
	}
	return req
}

func (r *Request) GetError() error {
	return r.err
}

func (r *Request) SetError(err error) RequestInterface {
	r.err = err
	return r
}

func (r *Request) SetContext(ctx context.Context) RequestInterface {
	r.span, _ = opentracing.StartSpanFromContext(ctx, "Quest: request")
	return r
}

func (r *Request) Header(key, value string) RequestInterface {
	if r.err != nil {
		return r
	}
	r.headers[key] = value
	return r
}

func (r *Request) QueryParam(key, value string) RequestInterface {
	if r.err != nil {
		return r
	}
	q := r.URL.Query()
	q.Add(key, value)
	r.URL.RawQuery = q.Encode()
	return r
}

func (r *Request) Param(key, value string) RequestInterface {
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

func (r *Request) Body(value *bytes.Buffer) RequestInterface {
	if r.err != nil {
		return r
	}
	r.data = value
	return r
}

func (r *Request) JSONBody(value interface{}) RequestInterface {
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

func (r *Request) MultipartBody(form *questmultipart.Form) RequestInterface {
	if r.err != nil {
		return r
	}
	r.Header("Content-Type", form.Writer.FormDataContentType())
	r.err = form.Err
	return r.Body(form.Buffer)
}

func (r *Request) Send() ResponseInterface {
	if r.err != nil {
		return &Response{
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
		return &Response{
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

	response, err := client.Do(req)
	if err != nil {
		r.err = handleRequestError(err, r)
		return &Response{
			Response: response,
			req:      r,
		}
	}

	return &Response{
		Response: response,
		req:      r,
	}
}

func (r *Response) GetRequest() RequestInterface {
	return r.req
}

func (r *Response) ExpectSuccess() ResponseInterface {
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

func (r *Response) ExpectStatusCode(code int) ResponseInterface {
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

func (r *Response) ExpectHeader(key, value string) ResponseInterface {
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

func (r *Response) ExpectType(value string) ResponseInterface {
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

func (r *Response) GetHeader(key string, into *string) ResponseInterface {
	if r.req.err != nil {
		return r
	}
	*into = r.Response.Header.Get(key)
	return r
}

func (r *Response) PrintJSON() ResponseInterface {
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

func (r *Response) GetBody(into *string) ResponseInterface {
	if r.req.err != nil {
		return r
	}
	defer r.Response.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Response.Body)
	*into = buf.String()
	return r
}

func (r *Response) GetJSON(into interface{}) ResponseInterface {
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

func (r *Response) Next() NextInterface {
	return &Next{&Client{}, r.req.err}
}

func (r *Response) Done() error {
	return r.req.err
}

func (r *Request) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(RequestJSON{
		r.URL,
		r.method,
		string(r.data.Bytes()),
		r.headers,
	}, "", "  ")
}

func (r *Request) UnmarshalJSON(b []byte) error {
	temp := &RequestJSON{}
	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	r.URL = temp.URL
	r.method = temp.Method
	r.data = bytes.NewBuffer([]byte(temp.Data))
	r.headers = temp.Headers

	return nil
}

type RequestJSON struct {
	*url.URL
	Method  string
	Data    string
	Headers map[string]string
}

type ResponseJSON struct {
	StatusCode    int
	Header        http.Header
	Body          string
	ContentLength int64
}

func (r *Response) MarshalJSON() ([]byte, error) {
	body, _ := ioutil.ReadAll(r.Response.Body)
	return json.MarshalIndent(ResponseJSON{
		r.Response.StatusCode,
		r.Response.Header,
		string(body),
		r.Response.ContentLength,
	}, "", "  ")
}

func (r *Response) UnmarshalJSON(b []byte) error {
	// not implemented
	return nil
}

func (r *Request) debug() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func (r *Response) debug() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func handleRequestError(err error, req *Request) *RequestError {
	return &RequestError{
		message: err.Error(),
		Request: req,
	}
}

func handleResponseError(err error, req *Request, resp *Response) *ResponseError {
	return &ResponseError{
		message:  err.Error(),
		Request:  req,
		Response: resp,
	}
}
