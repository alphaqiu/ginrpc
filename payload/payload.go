package payload

import "net/http"

type Response interface {
	GetCode() int
	GetErr() error
	GetHeader() http.Header
}

type DefaultResponse struct {
	Code   int
	Err    error
	Header http.Header
}

func (d *DefaultResponse) GetCode() int {
	return d.Code
}

func (d *DefaultResponse) GetErr() error {
	return d.Err
}

func (d *DefaultResponse) GetHeader() http.Header {
	return d.Header
}
