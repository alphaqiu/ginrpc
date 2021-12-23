package ginrpc

import "github.com/pkg/errors"

var (
	invalidInstanceErr = errors.New("无效的服务实例，服务实例必须是结构体指针")
)
