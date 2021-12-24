package ginrpc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alphaqiu/ginrpc/middleware/gzip"
	"github.com/alphaqiu/ginrpc/middleware/not_found"
	"github.com/alphaqiu/ginrpc/mock/request"
	"github.com/alphaqiu/ginrpc/mock/services/inventory"
	logging "github.com/ipfs/go-log/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"testing"
	"time"

	ginLogging "github.com/alphaqiu/ginrpc/middleware/logging"
	"github.com/alphaqiu/ginrpc/middleware/recover"
)

type testReq struct {
	method string
	url    string
	body   io.Reader
	header http.Header
}

var (
	requests = []testReq{
		{method: http.MethodPost, url: "/api/v1/inventory/add", body: bytes.NewBufferString(`{"name": "alpha"}`), header: nil},
		{method: http.MethodPost, url: "/api/v1/inventory/remove?name=tom", body: nil, header: nil},
		{method: http.MethodGet, url: "/api/v1/inventory/remove?name=jerry", body: nil, header: nil},
		{method: http.MethodGet, url: "/api/v1/inventory/data?name=octopus", body: nil, header: nil},
		{method: http.MethodGet, url: "/api/v1/inventory/empty?name=octopus", body: nil, header: nil},
		{method: http.MethodOptions, url: "/api/v1/inventory/empty?name=octopus", body: nil, header: nil},
		{method: http.MethodPost, url: "/api/v1/inventory/query?name=bruce", body: bytes.NewBufferString(`{"name": "alpha"}`), header: nil},
		{method: http.MethodPost, url: "/api/v1/inventory/revert?name=bruce", body: bytes.NewBufferString(`{"name": "alpha"}`), header: nil},
		{method: http.MethodPost, url: "/api/v1/inventory/header?name=bruce", body: bytes.NewBufferString(`{"name": "alpha"}`), header: http.Header{"x-lab": []string{"wow"}}},
		{method: http.MethodPost, url: "/api/v1/inventory/list?name=bruce", body: bytes.NewBufferString(`{"name": "alpha"}`), header: nil},
	}
)

func TestGinServer_Bind(t *testing.T) {
	_ = logging.SetLogLevel("*", "debug")
	httpServer := New(nil)
	if err := httpServer.Bind(&inventory.Inventory{}); err != nil {
		t.Fatalf("绑定服务失败: %v", err)
	}

	server := httpServer.(*ginServer)
	server.router.Use(ginLogging.Log(time.RFC3339, true), recover.Recover(true))
	server.makeRoutes()
	server.router.Use(not_found.NotFound(nil), gzip.Gzip(gzip.BestCompression))

	for _, r := range requests {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		quit := make(chan struct{})
		go func(q chan struct{}) {
			req := request.NewMockRequest(r.method, r.url, r.body, r.header)
			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)
			result, err := httputil.DumpResponse(w.Result(), true)
			if err != nil {
				fmt.Printf("请求失败: %+v\n", err)
			}
			fmt.Printf("Result: %s\n", result)
			close(quit)
		}(quit)

		select {
		case <-quit:
			t.Logf("正常退出")
		case <-ctx.Done():
			t.Logf("超时")
		}

		cancel()
	}
}
