package ginrpc

import (
	"ginrpc/payload"
	"github.com/gin-gonic/gin"
	"time"
)

const (
	DefaultReadTimeout  = 5 * time.Minute
	DefaultWriteTimeout = time.Minute
	DefaultIdleTimeout  = 2 * time.Minute
)

func defaultConfig() *Config {
	return &Config{
		Addr:        "127.0.0.1:12886",
		RunMode:     "debug",
		KeepAlive:   true,
		ReadTimeout: DefaultReadTimeout,
		WriteTimout: DefaultWriteTimeout,
		IdleTimeout: DefaultIdleTimeout,
		UrlPrefix:   "/api/v0",
	}
}

type Config struct {
	Addr                string        `mapstructure:"addr"`
	RunMode             string        `mapstructure:"mode"`
	KeepAlive           bool          `mapstructure:"keep_alive"`
	Tls                 *HttpTls      `mapstructure:"tls"`
	ReadTimeout         time.Duration `mapstructure:"read_timeout"`
	WriteTimout         time.Duration `mapstructure:"write_timeout"`
	IdleTimeout         time.Duration `mapstructure:"idle_timeout"`
	UrlPrefix           string        `mapstructure:"url_prefix"`
	ErrResponseFunc     func(ctx *gin.Context, resp payload.Response, data interface{})
	SuccessResponseFunc func(ctx *gin.Context, resp payload.Response, data interface{})
}

type HttpTls struct {
	Enabled  bool             `mapstructure:"enabled"`
	Redirect string           `mapstructure:"http_redirect"`
	CertFile string           `mapstructure:"cert_file"`
	KeyFile  string           `mapstructure:"key_file"`
	AutoCert *HttpTlsAutoCert `mapstructure:"autocert"`
}

type HttpTlsAutoCert struct {
	CertCache string   `mapstructure:"cert_cache"`
	Email     string   `mapstructure:"email"`
	Domains   []string `mapstructure:"domains"`
}
