package portal

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/soundadam/njulogin/internal/usage"
)

const (
	// DefaultBaseURL is the NJU authentication portal.
	DefaultBaseURL = "https://p.nju.edu.cn"
	// MaxResponseBytes bounds every portal and CAS response body.
	MaxResponseBytes = 1 << 20
)

// UserAgent is sent with every portal and CAS request; the portal serves its
// JSON API to browser user agents.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// ValidateBaseURL accepts an empty value, an HTTPS URL, or an HTTP loopback
// test server. Embedded credentials, queries, and fragments are rejected.
func ValidateBaseURL(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return usage.Errorf("invalid portal base URL: %v", err)
	}
	if parsed.User != nil {
		return usage.Errorf("portal base URL must not contain embedded credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return usage.Errorf("portal base URL must not contain a query or fragment")
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return nil
	}
	if parsed.Scheme == "http" && parsed.Host != "" {
		host := parsed.Hostname()
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return usage.Errorf("portal base URL must use https, except for an HTTP loopback test server")
}

// Do sends req and returns a bounded 2xx body. Errors never include the request
// URL, which may carry a QR token.
func Do(client *http.Client, req *http.Request) ([]byte, error) {
	return send(client, req, 299)
}

// DoWithoutRedirects sends req without following redirects and accepts 2xx and
// 3xx responses. Cookies set by the redirect response stay in the client's jar.
func DoWithoutRedirects(client *http.Client, req *http.Request) ([]byte, error) {
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return send(&clone, req, 399)
}

func send(client *http.Client, req *http.Request, maxStatus int) ([]byte, error) {
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("HTTP request failed")
	}
	defer resp.Body.Close()
	data, err := ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > maxStatus {
		return nil, fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
	}
	return data, nil
}

// ReadBody reads at most MaxResponseBytes and rejects anything larger.
func ReadBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxResponseBytes {
		return nil, fmt.Errorf("HTTP response exceeds %d bytes", MaxResponseBytes)
	}
	return data, nil
}
