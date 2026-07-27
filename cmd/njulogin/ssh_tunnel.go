package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

type sshTunnelAddr struct {
	network string
	value   string
}

func (addr sshTunnelAddr) Network() string { return addr.network }
func (addr sshTunnelAddr) String() string  { return addr.value }

type sshTunnelConn struct {
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	cancel     context.CancelFunc
	wait       <-chan error
	localAddr  net.Addr
	remoteAddr net.Addr
	closeOnce  sync.Once
	closeErr   error
}

func newSSHHTTPClient(host string, settings remoteSettings) (*http.Client, error) {
	if err := validateSSHHost(host); err != nil {
		return nil, err
	}
	if err := validateRemoteSettings(settings); err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialSSHTunnel(ctx, host, network, address, settings.ConnectTimeout)
		},
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: settings.Insecure, // #nosec G402 -- explicit diagnostic flag, warned by the caller.
		},
		ForceAttemptHTTP2: false,
	}
	return &http.Client{
		Timeout:   time.Duration(settings.MaxTime) * time.Second,
		Jar:       jar,
		Transport: transport,
	}, nil
}

func dialSSHTunnel(ctx context.Context, host, network, address string, connectTimeout int) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported SSH tunnel network %q", network)
	}
	if err := validateSSHHost(host); err != nil {
		return nil, err
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return nil, fmt.Errorf("invalid SSH tunnel target: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	processContext, cancel := context.WithCancel(context.Background())
	cmd := newCommandContext(
		processContext,
		"ssh",
		"-T",
		"-o", "ClearAllForwardings=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", connectTimeout),
		"-W", address,
		host,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start SSH tunnel: %w", err)
	}
	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
		close(wait)
	}()

	return &sshTunnelConn{
		stdin:      stdin,
		stdout:     stdout,
		cancel:     cancel,
		wait:       wait,
		localAddr:  sshTunnelAddr{network: "ssh", value: host},
		remoteAddr: sshTunnelAddr{network: network, value: address},
	}, nil
}

func (conn *sshTunnelConn) Read(buffer []byte) (int, error) {
	return conn.stdout.Read(buffer)
}

func (conn *sshTunnelConn) Write(buffer []byte) (int, error) {
	return conn.stdin.Write(buffer)
}

func (conn *sshTunnelConn) Close() error {
	conn.closeOnce.Do(func() {
		_ = conn.stdin.Close()
		_ = conn.stdout.Close()
		conn.cancel()
		select {
		case <-conn.wait:
		case <-time.After(time.Second):
			conn.closeErr = errors.New("SSH tunnel process did not stop promptly")
		}
	})
	return conn.closeErr
}

func (conn *sshTunnelConn) LocalAddr() net.Addr  { return conn.localAddr }
func (conn *sshTunnelConn) RemoteAddr() net.Addr { return conn.remoteAddr }

func (conn *sshTunnelConn) SetDeadline(deadline time.Time) error {
	readErr := conn.SetReadDeadline(deadline)
	writeErr := conn.SetWriteDeadline(deadline)
	return errors.Join(readErr, writeErr)
}

func (conn *sshTunnelConn) SetReadDeadline(deadline time.Time) error {
	if setter, ok := conn.stdout.(interface{ SetReadDeadline(time.Time) error }); ok {
		return setter.SetReadDeadline(deadline)
	}
	return nil
}

func (conn *sshTunnelConn) SetWriteDeadline(deadline time.Time) error {
	if setter, ok := conn.stdin.(interface{ SetWriteDeadline(time.Time) error }); ok {
		return setter.SetWriteDeadline(deadline)
	}
	return nil
}
