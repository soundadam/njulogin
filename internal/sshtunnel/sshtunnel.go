// Package sshtunnel adapts OpenSSH -W direct-tcpip stdio channels into
// net.Conn values. Nothing runs on the remote host; the SSH server only
// forwards bytes.
package sshtunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/soundadam/njulogin/internal/proc"
	"github.com/soundadam/njulogin/internal/usage"
)

// ValidateHost rejects SSH destinations that could be parsed as options or
// split into several arguments.
func ValidateHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return usage.Errorf("missing SSH host")
	}
	if strings.IndexFunc(host, func(value rune) bool {
		return value <= ' ' || value == 0x7f
	}) >= 0 {
		return usage.Errorf("SSH host must not contain whitespace or control characters")
	}
	if strings.HasPrefix(host, "-") {
		return usage.Errorf("SSH host must not begin with '-'")
	}
	return nil
}

// Dialer opens one `ssh -W` process per connection through the operator's SSH
// client and configuration.
type Dialer struct {
	Host string
	// ConnectTimeout is passed to ssh as ConnectTimeout, in seconds.
	ConnectTimeout int
}

// DialContext starts `ssh -W address Host` and returns its stdio as a conn.
func (d Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported SSH tunnel network %q", network)
	}
	if err := ValidateHost(d.Host); err != nil {
		return nil, err
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return nil, fmt.Errorf("invalid SSH tunnel target: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// The process outlives the dial context: it is the connection, and Close
	// ends it.
	processContext, cancel := context.WithCancel(context.Background())
	cmd := proc.Command(
		processContext,
		"ssh",
		"-T",
		"-o", "ClearAllForwardings=yes",
		"-o", "ExitOnForwardFailure=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", d.ConnectTimeout),
		"-W", address,
		d.Host,
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

	return &conn{
		stdin:      stdin,
		stdout:     stdout,
		cancel:     cancel,
		wait:       wait,
		localAddr:  addr{network: "ssh", value: d.Host},
		remoteAddr: addr{network: network, value: address},
	}, nil
}

type addr struct {
	network string
	value   string
}

func (a addr) Network() string { return a.network }
func (a addr) String() string  { return a.value }

type conn struct {
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	cancel     context.CancelFunc
	wait       <-chan error
	localAddr  net.Addr
	remoteAddr net.Addr
	closeOnce  sync.Once
	closeErr   error
}

func (c *conn) Read(buffer []byte) (int, error)  { return c.stdout.Read(buffer) }
func (c *conn) Write(buffer []byte) (int, error) { return c.stdin.Write(buffer) }

func (c *conn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		c.cancel()
		select {
		case <-c.wait:
		case <-time.After(time.Second):
			c.closeErr = errors.New("SSH tunnel process did not stop promptly")
		}
	})
	return c.closeErr
}

func (c *conn) LocalAddr() net.Addr  { return c.localAddr }
func (c *conn) RemoteAddr() net.Addr { return c.remoteAddr }

func (c *conn) SetDeadline(deadline time.Time) error {
	return errors.Join(c.SetReadDeadline(deadline), c.SetWriteDeadline(deadline))
}

func (c *conn) SetReadDeadline(deadline time.Time) error {
	if setter, ok := c.stdout.(interface{ SetReadDeadline(time.Time) error }); ok {
		return setter.SetReadDeadline(deadline)
	}
	return nil
}

func (c *conn) SetWriteDeadline(deadline time.Time) error {
	if setter, ok := c.stdin.(interface{ SetWriteDeadline(time.Time) error }); ok {
		return setter.SetWriteDeadline(deadline)
	}
	return nil
}
