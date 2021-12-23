package request

import (
	"io"
	"net/http"
	"net/http/httptest"
)

func NewMockRequest(method, url string, body io.Reader, header http.Header) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json; charset=utf8")
	if header != nil {
		for k, v := range header {
			for _, item := range v {
				req.Header.Add(k, item)
			}
		}
	}
	return req
}
