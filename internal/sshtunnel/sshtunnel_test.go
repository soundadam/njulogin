package sshtunnel

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njulogin/internal/testutil/fakessh"
	"github.com/soundadam/njulogin/internal/usage"
)

func TestValidateHost(t *testing.T) {
	if err := ValidateHost("synthetic-host"); err != nil {
		t.Fatalf("ValidateHost: %v", err)
	}
	for _, host := range []string{"", "-oProxyCommand=synthetic", "synthetic host", "synthetic\nhost"} {
		if err := ValidateHost(host); !usage.Is(err) {
			t.Errorf("ValidateHost(%q) = %v, want usage error", host, err)
		}
	}
}

func TestDialRejectsUnsupportedInput(t *testing.T) {
	dialer := Dialer{Host: "synthetic-host", ConnectTimeout: 5}
	if _, err := dialer.DialContext(context.Background(), "udp", "127.0.0.1:1"); err == nil {
		t.Fatal("dialed a non-TCP network")
	}
	if _, err := dialer.DialContext(context.Background(), "tcp", "no-port"); err == nil {
		t.Fatal("dialed a target without a port")
	}
}

func TestConnLifecycleOverFakeSSH(t *testing.T) {
	argsLog := fakessh.Install(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "synthetic-body")
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")

	conn, err := Dialer{Host: "synthetic-host", ConnectTimeout: 5}.DialContext(context.Background(), "tcp", address)
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	if conn.LocalAddr().Network() != "ssh" || conn.RemoteAddr().String() != address {
		t.Fatalf("addresses local=%v remote=%v", conn.LocalAddr(), conn.RemoteAddr())
	}
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	if err := request.Write(conn); err != nil {
		t.Fatalf("Write: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "synthetic-body" {
		t.Fatalf("body = %q", body)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "-T\n-o\nClearAllForwardings=yes\n-o\nExitOnForwardFailure=yes\n-o\nConnectTimeout=5\n-W\n" + address + "\nsynthetic-host\n"
	if string(args) != want {
		t.Fatalf("ssh args = %q", args)
	}
}
