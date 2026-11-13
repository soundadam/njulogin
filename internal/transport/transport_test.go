package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
)

type refusingDialer struct{}

func (refusingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("synthetic refusal")
}

func TestClientsIgnoreAmbientProxySettings(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	for _, opts := range []Options{{}, {Dialer: refusingDialer{}}} {
		client, err := NewHTTPClient(opts)
		if err != nil {
			t.Fatalf("NewHTTPClient: %v", err)
		}
		transport, ok := client.Transport.(*http.Transport)
		if !ok || transport.Proxy != nil || client.Jar == nil {
			t.Fatalf("transport = %#v", client.Transport)
		}
		if transport.TLSClientConfig.MinVersion < 0x0303 {
			t.Fatal("TLS minimum is below 1.2")
		}
	}
}

func TestDialedConnectionsAreNotReused(t *testing.T) {
	client, err := NewHTTPClient(Options{Dialer: refusingDialer{}})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	if !client.Transport.(*http.Transport).DisableKeepAlives {
		t.Fatal("dialed connections must close after each request")
	}
	if _, err := client.Get("http://127.0.0.1:1/"); err == nil {
		t.Fatal("request bypassed the dialer")
	}
}
