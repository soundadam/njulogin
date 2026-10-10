// Package cas drives the NJU CAS QR-code login. The CAS page, token, QR
// rendering, and status polling are all handled in this process.
package cas

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/usage"
)

// Endpoints locates the CAS server and the portal service it logs in to.
type Endpoints struct {
	BaseURL string
	Service string
}

// DefaultEndpoints is the NJU CAS server issuing portal tickets.
var DefaultEndpoints = Endpoints{
	BaseURL: "https://authserver.nju.edu.cn/authserver",
	Service: "https://p.nju.edu.cn:443/api/cas/getinfo/&renew=true",
}

// Renderer draws the QR payload for the operator. The payload is a login URL
// and must stay inside this process.
type Renderer interface {
	Render(w io.Writer, payload string) error
}

// TerminalQR renders a compact QR code with Unicode half blocks.
type TerminalQR struct{}

// Render writes the QR code to w.
func (TerminalQR) Render(w io.Writer, payload string) error {
	qr, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		return errors.New("generate QR code from portal token")
	}
	_, err = io.WriteString(w, qr.ToSmallString(false))
	return err
}

// QRLogin performs one QR login and reports progress as scan_state records.
type QRLogin struct {
	HTTP         *http.Client
	Endpoints    Endpoints
	Renderer     Renderer
	Out          io.Writer
	Timeout      time.Duration
	PollInterval time.Duration
}

// Fields are the hidden inputs of the CAS login form.
type Fields struct {
	LT        string
	DLLT      string
	Execution string
	EventID   string
	RMShown   string
}

// Run returns after the CAS login form is submitted. The caller verifies the
// resulting portal state.
func (q QRLogin) Run(ctx context.Context) error {
	if err := portal.ValidateBaseURL(q.Endpoints.BaseURL); err != nil {
		return usage.Errorf("invalid CAS base URL: %v", err)
	}
	base := strings.TrimRight(q.Endpoints.BaseURL, "/")
	loginURL := base + "/login?service=" + url.QueryEscape(q.Endpoints.Service)
	page, err := q.get(ctx, loginURL)
	if err != nil {
		return fmt.Errorf("load CAS login page: %w", err)
	}
	fields := ParseFields(string(page))
	if fields.LT == "" || fields.Execution == "" || fields.EventID == "" {
		return errors.New("CAS login page is missing required form fields")
	}
	tokenData, err := q.get(ctx, fmt.Sprintf("%s/qrCode/getToken?ts=%d", base, time.Now().UnixMilli()))
	if err != nil {
		return fmt.Errorf("request QR token: %w", err)
	}
	token := strings.TrimSpace(string(tokenData))
	if token == "" {
		return errors.New("empty QR token")
	}
	if err := q.Renderer.Render(q.Out, fmt.Sprintf("%s/qrCode/qrCodeLogin.do?uuid=%s", base, url.QueryEscape(token))); err != nil {
		return err
	}
	fmt.Fprintln(q.Out, "scan_state=waiting")
	deadline := time.Now().Add(q.Timeout)
	confirmedReported := false
	for time.Now().Before(deadline) {
		state, err := q.get(ctx, fmt.Sprintf("%s/qrCode/getStatus.htl?ts=%d&uuid=%s", base, time.Now().UnixMilli(), url.QueryEscape(token)))
		if err != nil {
			return fmt.Errorf("poll QR status: %w", err)
		}
		switch strings.TrimSpace(string(state)) {
		case "1":
			return q.submit(ctx, loginURL, fields, token)
		case "2":
			if !confirmedReported {
				fmt.Fprintln(q.Out, "scan_state=confirmed_on_phone_pending")
				confirmedReported = true
			}
		case "3":
			return errors.New("QR code expired")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(q.PollInterval):
		}
	}
	return errors.New("QR login timed out")
}

func (q QRLogin) get(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return portal.Do(q.HTTP, req)
}

func (q QRLogin) submit(ctx context.Context, loginURL string, fields Fields, token string) error {
	form := url.Values{}
	form.Set("lt", fields.LT)
	form.Set("uuid", token)
	form.Set("cllt", "qrLogin")
	form.Set("dllt", fields.DLLT)
	form.Set("execution", fields.Execution)
	form.Set("_eventId", fields.EventID)
	form.Set("rmShown", fields.RMShown)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := portal.DoWithoutRedirects(q.HTTP, req); err != nil {
		return fmt.Errorf("submit QR login: %w", err)
	}
	fmt.Fprintln(q.Out, "scan_state=login_submitted")
	return nil
}

// ParseFields extracts the hidden CAS form inputs.
func ParseFields(document string) Fields {
	return Fields{
		LT:        inputValue(document, "lt"),
		DLLT:      inputValue(document, "dllt"),
		Execution: inputValue(document, "execution"),
		EventID:   inputValue(document, "_eventId"),
		RMShown:   inputValue(document, "rmShown"),
	}
}

var valuePattern = regexp.MustCompile(`(?is)value=["']([^"']*)["']`)

func inputValue(document, name string) string {
	pattern := regexp.MustCompile(`(?is)<input[^>]+name=["']` + regexp.QuoteMeta(name) + `["'][^>]*>`)
	tag := pattern.FindString(document)
	if tag == "" {
		return ""
	}
	match := valuePattern.FindStringSubmatch(tag)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(match[1])
}
