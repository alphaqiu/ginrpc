package ginrpc

import (
	"crypto/tls"
	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

var (
	errInterface = reflect.TypeOf((*error)(nil)).Elem()
)

// Redirect HTTP requests to HTTPS
func tlsRedirect(toPort string) http.HandlerFunc {
	if toPort == ":443" || toPort == ":https" {
		toPort = ""
	} else if toPort != "" && toPort[:1] == ":" {
		// Strip leading colon. JoinHostPort will add it back.
		toPort = toPort[1:]
	}

	return func(wrt http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			// If SplitHostPort has failed assume it's because :port part is missing.
			host = req.Host
		}

		target, _ := url.ParseRequestURI(req.RequestURI)
		target.Scheme = "https"

		// Ensure valid redirect target.
		if toPort != "" {
			// Replace the port number.
			target.Host = net.JoinHostPort(host, toPort)
		} else {
			target.Host = host
		}

		if target.Path == "" {
			target.Path = "/"
		}

		http.Redirect(wrt, req, target.String(), http.StatusTemporaryRedirect)
	}
}

func makeTls(cnf *HttpTls) *tls.Config {
	if cnf.AutoCert != nil && len(cnf.AutoCert.CertCache) > 0 {
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cnf.AutoCert.Domains...),
			Cache:      autocert.DirCache(cnf.AutoCert.CertCache),
			Email:      cnf.AutoCert.Email,
		}

		return certManager.TLSConfig()
	}

	// Otherwise, try to use static keys.
	cert, err := tls.LoadX509KeyPair(cnf.CertFile, cnf.KeyFile)
	if err != nil {
		panic(errors.Wrap(err, "load tls key & cert failed. pls check them correctly"))
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

// return http method name/resource name/ action name
func parseMethodName(resource reflect.Type, method string) (string, string, string) {
	check := strings.HasPrefix(method, "Get")
	if check {
		methodName := strings.ToLower(method[3:])
		return http.MethodGet, strings.ToLower(resource.Name()), methodName
	}

	check = strings.HasPrefix(method, "Options")
	if check {
		methodName := strings.ToLower(method[7:])
		return http.MethodOptions, strings.ToLower(resource.Name()), methodName
	}

	return http.MethodPost, strings.ToLower(resource.Name()), strings.ToLower(method)
}
