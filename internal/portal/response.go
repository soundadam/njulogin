package portal

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Response is the envelope every portal JSON endpoint returns.
type Response struct {
	ReplyCode int             `json:"reply_code"`
	Results   json.RawMessage `json:"results"`
}

// Session is one authenticated session reported by getinfo.
type Session struct {
	Username      string `json:"username"`
	Fullname      string `json:"fullname"`
	MAC           string `json:"mac"`
	UserIPv6      string `json:"user_ipv6"`
	ServiceName   string `json:"service_name"`
	AcctStartTime int64  `json:"acctstarttime"`
}

// Status is the authentication state of the address the request came from.
type Status struct {
	Authenticated bool
	Sessions      []Session
}

// Row is one online-device or quick-login record. Rows stay untyped because
// the portal returns ids as either strings or numbers.
type Row map[string]any

// ID is the record identifier as the portal formats it.
func (r Row) ID() string { return strings.TrimSpace(fmt.Sprint(r["id"])) }

// Field formats one raw field; missing fields format as "<nil>".
func (r Row) Field(name string) string { return fmt.Sprint(r[name]) }

// ParseResponse decodes the envelope and requires an integer reply_code.
func ParseResponse(data []byte) (*Response, error) {
	var wire struct {
		ReplyCode *int            `json:"reply_code"`
		Results   json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	if wire.ReplyCode == nil {
		return nil, errors.New("portal response is missing reply_code")
	}
	return &Response{ReplyCode: *wire.ReplyCode, Results: wire.Results}, nil
}

// RequireSuccess accepts an operation response only when reply_code is 0.
func RequireSuccess(data []byte) error {
	response, err := ParseResponse(data)
	if err != nil {
		return fmt.Errorf("parse portal operation response: %w", err)
	}
	if response.ReplyCode != 0 {
		return fmt.Errorf("portal operation rejected with reply_code=%d", response.ReplyCode)
	}
	return nil
}

// ParseStatus interprets a getinfo response. A nonzero reply_code or an
// explicit empty rows list means unauthenticated; missing rows are malformed.
func ParseStatus(response *Response) (Status, error) {
	if response.ReplyCode != 0 {
		return Status{}, nil
	}
	var results struct {
		Rows *[]Session `json:"rows"`
	}
	if err := json.Unmarshal(response.Results, &results); err != nil {
		return Status{}, err
	}
	if results.Rows == nil {
		return Status{}, errors.New("portal status response is missing results.rows")
	}
	if len(*results.Rows) == 0 {
		return Status{}, nil
	}
	return Status{Authenticated: true, Sessions: *results.Rows}, nil
}

// ParseQuickRows interprets a quick-login listing; reply_code 404 means none.
func ParseQuickRows(response *Response) ([]Row, error) {
	if response.ReplyCode == 404 {
		return []Row{}, nil
	}
	return parseRows(response, "quick-login")
}

// ParseOnlineRows interprets an online-session listing.
func ParseOnlineRows(response *Response) ([]Row, error) {
	return parseRows(response, "online-session")
}

func parseRows(response *Response, kind string) ([]Row, error) {
	if response.ReplyCode != 0 {
		return nil, fmt.Errorf("portal %s query failed with reply_code=%d", kind, response.ReplyCode)
	}
	var results struct {
		Rows *[]Row `json:"rows"`
	}
	if err := json.Unmarshal(response.Results, &results); err != nil {
		return nil, err
	}
	if results.Rows == nil {
		return nil, fmt.Errorf("portal %s response is missing results.rows", kind)
	}
	return *results.Rows, nil
}
