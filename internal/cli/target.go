package cli

import (
	"net/http"
	"os"
	"time"

	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/sshtunnel"
	"github.com/soundadam/njulogin/internal/transport"
	"github.com/soundadam/njulogin/internal/usage"
)

const (
	maxConnectTimeout = 300
	maxRequestTimeout = 3600
	localTimeout      = 15 * time.Second
)

// Target is the machine whose network address the portal sees. Every command
// runs the same way against any Target; only the transport differs.
type Target interface {
	// HTTPClient returns a fresh client whose connections originate at the target.
	HTTPClient() (*http.Client, error)
	PortalURL() string
	// Prefix labels flag sets and errors: "" locally, "remote " over SSH.
	Prefix() string
	// Invocation is the command prefix shown in usage hints.
	Invocation() string
	// DeviceName is the default quick-login device name.
	DeviceName() string
}

func newAPI(target Target) (portal.API, *http.Client, error) {
	client, err := target.HTTPClient()
	if err != nil {
		return nil, nil, err
	}
	api, err := portal.New(target.PortalURL(), client)
	if err != nil {
		return nil, nil, err
	}
	return api, client, nil
}

// Local reaches the portal directly from this machine.
type Local struct {
	BaseURL string
}

func (Local) HTTPClient() (*http.Client, error) {
	return transport.NewHTTPClient(transport.Options{Timeout: localTimeout})
}
func (l Local) PortalURL() string { return l.BaseURL }
func (Local) Prefix() string      { return "" }
func (Local) Invocation() string  { return "njulogin" }
func (Local) DeviceName() string {
	hostname, _ := os.Hostname()
	return hostname
}

// RemoteSettings are the `remote` flags that precede the SSH host.
type RemoteSettings struct {
	BaseURL        string
	Insecure       bool
	ConnectTimeout int
	MaxTime        int
}

func defaultRemoteSettings(baseURL string) RemoteSettings {
	return RemoteSettings{BaseURL: baseURL, ConnectTimeout: 5, MaxTime: 15}
}

func (s RemoteSettings) validate() error {
	if err := portal.ValidateBaseURL(s.BaseURL); err != nil {
		return err
	}
	if s.ConnectTimeout <= 0 {
		return usage.Errorf("--connect-timeout must be positive")
	}
	if s.ConnectTimeout > maxConnectTimeout {
		return usage.Errorf("--connect-timeout must be at most %d seconds", maxConnectTimeout)
	}
	if s.MaxTime <= 0 {
		return usage.Errorf("--max-time must be positive")
	}
	if s.MaxTime > maxRequestTimeout {
		return usage.Errorf("--max-time must be at most %d seconds", maxRequestTimeout)
	}
	return nil
}

// SSH reaches the portal through an `ssh -W` tunnel from Host. HTTP, TLS, and
// cookies stay local.
type SSH struct {
	Host     string
	Settings RemoteSettings
}

func (s SSH) HTTPClient() (*http.Client, error) {
	if err := sshtunnel.ValidateHost(s.Host); err != nil {
		return nil, err
	}
	if err := s.Settings.validate(); err != nil {
		return nil, err
	}
	return transport.NewHTTPClient(transport.Options{
		Dialer:             sshtunnel.Dialer{Host: s.Host, ConnectTimeout: s.Settings.ConnectTimeout},
		Timeout:            time.Duration(s.Settings.MaxTime) * time.Second,
		InsecureSkipVerify: s.Settings.Insecure,
	})
}
func (s SSH) PortalURL() string  { return s.Settings.BaseURL }
func (SSH) Prefix() string       { return "remote " }
func (SSH) Invocation() string   { return "njulogin remote <host>" }
func (s SSH) DeviceName() string { return s.Host }
