package quest

import "net/http"

// Next is used to chain requests together
type Next struct {
	err error
}

// New creates a new request with given http method and path (uri) and is
// used when chaining requests together
func (n *Next) New(method, path string) *Request {
	req := New(method, path)
	if req.err == nil {
		req.err = n.err
	}
	return req
}

// Get creates a new http "GET" request for path (uri) and is used when chaining requests together
func (n *Next) Get(path string) *Request {
	return n.New(http.MethodGet, path)
}

// Post creates a new http "POST" request for path (uri) and is used when chaining requests together
func (n *Next) Post(path string) *Request {
	return n.New(http.MethodPost, path)
}

// Put creates a new http "Put" request for path (uri) and is used when chaining requests together
func (n *Next) Put(path string) *Request {
	return n.New(http.MethodPut, path)
}

// Delete creates a new http "Delete" request for path (uri) and is used when chaining requests together
func (n *Next) Delete(path string) *Request {
	return n.New(http.MethodDelete, path)
}
