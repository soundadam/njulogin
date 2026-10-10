// Package transport builds the HTTP clients that carry portal and CAS traffic.
// HTTP, TLS, and cookies always run in this process; a Dialer only decides
// where the TCP byte stream originates.
package transport

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Dialer opens the byte stream for one HTTP connection.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Options configures an HTTP client. A nil Dialer connects directly from this
// machine.
type Options struct {
	Dialer             Dialer
	Timeout            time.Duration
	InsecureSkipVerify bool
}

// NewHTTPClient returns a client with an in-memory cookie jar, TLS 1.2 or
// newer, and no ambient proxy.
func NewHTTPClient(opts Options) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   opts.Timeout,
		Jar:       jar,
		Transport: newTransport(opts),
	}, nil
}

func newTransport(opts Options) *http.Transport {
	if opts.Dialer == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			transport.TLSClientConfig.MinVersion = tls.VersionTLS12
		}
		transport.TLSClientConfig.InsecureSkipVerify = opts.InsecureSkipVerify // #nosec G402 -- explicit diagnostic flag, warned by the caller.
		return transport
	}
	// A dialed connection may be a subprocess; closing it after each request
	// bounds how long that process lives.
	return &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
		DialContext:       opts.Dialer.DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opts.InsecureSkipVerify, // #nosec G402 -- explicit diagnostic flag, warned by the caller.
		},
		ForceAttemptHTTP2: false,
	}
}
