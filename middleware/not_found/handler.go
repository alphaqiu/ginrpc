package not_found

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func NotFound(message interface{}) gin.HandlerFunc {
	if message == nil {
		message = defaultMessage()
	}
	return func(c *gin.Context) {
		c.Abort()
		c.JSON(http.StatusNotFound, message)
	}
}

func defaultMessage() gin.H {
	return gin.H{"code": 404, "message": "API not found", "error": "Route not allowed"}
}
