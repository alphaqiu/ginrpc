package gzip

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const (
	BestCompression    = gzip.BestCompression
	BestSpeed          = gzip.BestSpeed
	DefaultCompression = gzip.DefaultCompression
	NoCompression      = gzip.NoCompression
)

var (
	WithExcludedExtensions  = gzip.WithExcludedExtensions
	WithExcludedPaths       = gzip.WithExcludedPaths
	WithExcludedPathsRegexs = gzip.WithExcludedPathsRegexs
	WithDecompressFn        = gzip.WithDecompressFn
)

func Gzip(level int, options ...gzip.Option) gin.HandlerFunc {
	return gzip.Gzip(level, options...)
}

func DefaultDecompressHandle() gin.HandlerFunc {
	return gzip.DefaultDecompressHandle
}
