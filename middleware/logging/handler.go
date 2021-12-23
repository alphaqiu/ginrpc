package logging

import (
	"github.com/gin-gonic/gin"
	logging "github.com/ipfs/go-log/v2"
	"time"
)

var log = logging.Logger("middleware")

func Log(timeFormat string, utc bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		if utc {
			end = end.UTC()
		}

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			for _, e := range c.Errors.Errors() {
				log.Errorf("ginError: %v", e)
			}
		} else {
			log.Info(path,
				"| status: ", c.Writer.Status(),
				"| method: ", c.Request.Method,
				"| path: ", path,
				"| query: ", query,
				"| IP: ", c.ClientIP(),
				"| user-agent: ", c.Request.UserAgent(),
				"| time: ", end.Format(timeFormat),
				"| latency: ", latency,
			)
		}
	}
}
