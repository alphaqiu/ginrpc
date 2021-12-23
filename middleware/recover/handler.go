package recover

import (
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("middleware")

func Recover(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					tpl := "[Recovery from panic] URLPath: %s, err: %s, request: %s"
					log.Errorf(tpl, c.Request.URL.Path, err, string(httpRequest))
					// If the connection is dead, we can't write a status to it.
					_ = c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				ts := time.Now().Format(time.RFC3339)
				if stack {
					tpl := "[Recovery from panic] time: %s, err: %s, request: %s, stack: %s"
					log.Errorf(tpl, ts, err, string(httpRequest), string(debug.Stack()))
				} else {
					tpl := "[Recovery from panic] time: %s, err: %s, request: %s"
					log.Errorf(tpl, ts, err, string(httpRequest))
				}

				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}
