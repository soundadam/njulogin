// Package portal speaks the p.nju.edu.cn JSON API and verifies that accepted
// operations reach their documented postconditions.
package portal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StatusReader reports the current authentication state.
type StatusReader interface {
	Status(ctx context.Context) (Status, error)
}

// API is every portal operation the CLI performs. Operations return once the
// portal accepts them; Poll verifies their postconditions.
type API interface {
	StatusReader
	Login(ctx context.Context, username, password string) error
	Logout(ctx context.Context) error
	OnlineDevices(ctx context.Context) ([]Row, error)
	Kick(ctx context.Context, id string) error
	QuickDevices(ctx context.Context) ([]Row, error)
	BindQuick(ctx context.Context, name string) error
	UnbindQuick(ctx context.Context, id string) error
}

var _ API = (*Client)(nil)

// Client calls the portal at BaseURL through an HTTP client whose transport
// decides where the connection originates.
type Client struct {
	baseURL string
	http    *http.Client
}

// New validates baseURL and binds it to httpClient.
func New(baseURL string, httpClient *http.Client) (*Client, error) {
	if err := ValidateBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}, nil
}

// Status reads getinfo for the requesting address.
func (c *Client) Status(ctx context.Context) (Status, error) {
	response, err := c.query(ctx, "/api/portal/v1/getinfo")
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(response)
}

// Login submits a password login for the requesting address.
func (c *Client) Login(ctx context.Context, username, password string) error {
	return c.operate(ctx, "/api/portal/v1/login", map[string]string{
		"username": username,
		"password": password,
	})
}

// Logout ends the requesting address's session.
func (c *Client) Logout(ctx context.Context) error {
	return c.operate(ctx, "/api/portal/v1/logout", map[string]string{"domain": "default"})
}

// OnlineDevices lists the account's online sessions.
func (c *Client) OnlineDevices(ctx context.Context) ([]Row, error) {
	response, err := c.query(ctx, "/api/selfservice/v1/online", "page=1", "limit=10000")
	if err != nil {
		return nil, err
	}
	return ParseOnlineRows(response)
}

// Kick ends the online session with the given id.
func (c *Client) Kick(ctx context.Context, id string) error {
	return c.operate(ctx, "/api/selfservice/v1/online/kick/"+url.PathEscape(id), map[string]string{})
}

// QuickDevices lists the account's quick-login bindings.
func (c *Client) QuickDevices(ctx context.Context) ([]Row, error) {
	response, err := c.query(ctx, "/api/portal/v1/quicklogin/domain/default")
	if err != nil {
		return nil, err
	}
	return ParseQuickRows(response)
}

// BindQuick binds the requesting device for quick login under name.
func (c *Client) BindQuick(ctx context.Context, name string) error {
	return c.operate(ctx, "/api/portal/v1/quicklogin/bind/default", map[string]string{"device": name})
}

// UnbindQuick removes the quick-login binding with the given id.
func (c *Client) UnbindQuick(ctx context.Context, id string) error {
	return c.operate(ctx, "/api/portal/v1/quicklogin/unbind/id/"+url.PathEscape(id), map[string]string{})
}

// query GETs path with a cache-busting timestamp appended after params.
func (c *Client) query(ctx context.Context, path string, params ...string) (*Response, error) {
	params = append(params, fmt.Sprintf("_=%d", time.Now().UnixMilli()))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+strings.Join(params, "&"), nil)
	if err != nil {
		return nil, err
	}
	data, err := Do(c.http, req)
	if err != nil {
		return nil, err
	}
	return ParseResponse(data)
}

func (c *Client) operate(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	reply, err := Do(c.http, req)
	if err != nil {
		return err
	}
	return RequireSuccess(reply)
}
