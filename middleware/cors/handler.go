package cors

import (
	"github.com/gin-gonic/gin"
	logging "github.com/ipfs/go-log/v2"
	"net/http"
	"net/http/httputil"
)

const (
	DefaultOrigin  = "*"
	DefaultMethods = "GET, POST, PUT, DELETE, OPTIONS"
	DefaultHeaders = "DNT,X-Mx-ReqToken,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization,X-Language,X-Api-Key,X-Api-Secret,Content-Disposition"
)

var log = logging.Logger("middleware")

func DefaultCors() gin.HandlerFunc {
	return Cors(DefaultOrigin, DefaultMethods, DefaultHeaders)
}

func Cors(origin, methods, headers string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		if http.MethodOptions == c.Request.Method {
			httpRequest, _ := httputil.DumpRequest(c.Request, false)
			log.Warnf("Cors Request Abort: %s", string(httpRequest))
			c.Abort()
			c.JSON(http.StatusOK, gin.H{})
		} else {
			c.Next()
		}
	}
}
