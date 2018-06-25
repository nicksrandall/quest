package quest

import "fmt"

type requestError struct {
	message string
	Request *Request
}

type responseError struct {
	message  string
	Request  *Request
	Response *Response
}

func (e requestError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s", e.message, e.Request.format())
}

func (e responseError) Error() string {
	return fmt.Sprintf("[Quest]: Request Error - %s\n\nRequest Info:\n %s\n\nResponse Info:\n %s", e.message, e.Request.format(), e.Response.format())
}

func handleRequestError(err error, req *Request) *requestError {
	return &requestError{
		message: err.Error(),
		Request: req,
	}
}

func handleResponseError(err error, req *Request, resp *Response) *responseError {
	return &responseError{
		message:  err.Error(),
		Request:  req,
		Response: resp,
	}
}
