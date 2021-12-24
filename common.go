package ginrpc

import (
	"github.com/pkg/errors"
	"net/http"
)

var (
	invalidInstanceErr = errors.New("无效的服务实例，服务实例必须是结构体指针")
)

type Err interface {
	Code() int
	Message() string
	Error() string
}

type internalError struct {
	error
}

func (e *internalError) Code() int {
	return http.StatusInternalServerError
}

func (e *internalError) Message() string {
	return "unknown server error"
}

func (e *internalError) Error() string {
	return e.error.Error()
}

type ResourceVersion interface {
	Version() string
}

type ResourceInterceptor interface {
	Before()
	After()
}
