package portal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/soundadam/njulogin/internal/usage"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestValidateBaseURL(t *testing.T) {
	for _, value := range []string{
		"",
		"https://p.nju.edu.cn",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	} {
		if err := ValidateBaseURL(value); err != nil {
			t.Errorf("ValidateBaseURL(%q): %v", value, err)
		}
	}
	for _, value := range []string{
		"http://portal.example.invalid",
		"https://user:secret@example.invalid",
		"https://example.invalid?token=synthetic",
		"https://example.invalid#fragment",
	} {
		if err := ValidateBaseURL(value); !usage.Is(err) {
			t.Errorf("ValidateBaseURL(%q) = %v, want usage error", value, err)
		}
	}
}

func TestDoWithoutRedirectsAccepts302AndPreservesCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/submit":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/missing", http.StatusFound)
		case "/api/portal/v1/getinfo":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != "ok" {
				http.Error(w, "missing session cookie", http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"123456789"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	httpClient := &http.Client{Jar: jar}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/submit", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if _, err := DoWithoutRedirects(httpClient, req); err != nil {
		t.Fatalf("DoWithoutRedirects: %v", err)
	}
	client, err := New(server.URL, httpClient)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Authenticated || len(status.Sessions) != 1 || status.Sessions[0].Username != "123456789" {
		t.Fatalf("status = %#v", status)
	}
}

func TestHTTPErrorsDoNotExposeTokenBearingURL(t *testing.T) {
	const token = "synthetic-private-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic upstream failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/status?uuid="+token, nil)
	_, err := Do(server.Client(), req)
	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("status error = %v", err)
	}

	failing := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New(request.URL.String())
	})}
	req, _ = http.NewRequest(http.MethodGet, "https://example.invalid/status?uuid="+token, nil)
	if _, err := Do(failing, req); err == nil || err.Error() != "HTTP request failed" {
		t.Fatalf("transport error = %v", err)
	}
}

func TestReadBodyRejectsOversizedPayload(t *testing.T) {
	if _, err := ReadBody(strings.NewReader(strings.Repeat("x", MaxResponseBytes+1))); err == nil {
		t.Fatal("expected oversized response error")
	}
	data, err := ReadBody(strings.NewReader(strings.Repeat("x", MaxResponseBytes)))
	if err != nil || len(data) != MaxResponseBytes {
		t.Fatalf("ReadBody at limit: len=%d err=%v", len(data), err)
	}
}

func TestRequireSuccess(t *testing.T) {
	if err := RequireSuccess([]byte(`{"reply_code":0}`)); err != nil {
		t.Fatalf("success response: %v", err)
	}
	for _, data := range []string{
		`{"reply_code":7,"reply_msg":"synthetic rejection"}`,
		`not-json`,
		`{}`,
		`{"reply_code":null}`,
		`{"reply_code":"0"}`,
	} {
		if err := RequireSuccess([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}

func TestParseStatus(t *testing.T) {
	status, err := ParseStatus(&Response{ReplyCode: 404, Results: json.RawMessage(`{}`)})
	if err != nil || status.Authenticated {
		t.Fatalf("nonzero reply_code: %#v, %v", status, err)
	}
	status, err = ParseStatus(&Response{Results: json.RawMessage(`{"rows":[]}`)})
	if err != nil || status.Authenticated {
		t.Fatalf("explicit empty rows: %#v, %v", status, err)
	}
	for _, results := range []string{`"not-an-object"`, `{}`, `{"rows":null}`} {
		if _, err := ParseStatus(&Response{Results: json.RawMessage(results)}); err == nil {
			t.Errorf("accepted malformed results %s", results)
		}
	}
}

func TestListResponsesRequireExplicitRows(t *testing.T) {
	for _, results := range []string{`{}`, `{"rows":null}`} {
		response := &Response{Results: json.RawMessage(results)}
		if _, err := ParseQuickRows(response); err == nil {
			t.Errorf("quick rows accepted %s", results)
		}
		if _, err := ParseOnlineRows(response); err == nil {
			t.Errorf("online rows accepted %s", results)
		}
	}
	empty := &Response{Results: json.RawMessage(`{"rows":[]}`)}
	if rows, err := ParseQuickRows(empty); err != nil || len(rows) != 0 {
		t.Fatalf("quick empty rows = %#v, %v", rows, err)
	}
	if rows, err := ParseOnlineRows(empty); err != nil || len(rows) != 0 {
		t.Fatalf("online empty rows = %#v, %v", rows, err)
	}
	wide := &Response{Results: json.RawMessage(`{"rows":[{"id":17733220524964001,"device":"synthetic"}]}`)}
	if rows, err := ParseQuickRows(wide); err != nil || rows[0].ID() != "17733220524964001" {
		t.Fatalf("wide numeric id = %#v, %v", rows, err)
	}
	if rows, err := ParseQuickRows(&Response{ReplyCode: 404}); err != nil || len(rows) != 0 {
		t.Fatalf("quick 404 = %#v, %v", rows, err)
	}
	if _, err := ParseOnlineRows(&Response{ReplyCode: 404}); err == nil {
		t.Fatal("online listing accepted reply_code 404")
	}
}

type fixedStatus Status

func (s fixedStatus) Status(context.Context) (Status, error) { return Status(s), nil }

func TestPollAuthStateTimesOut(t *testing.T) {
	_, err := Poll{}.AuthState(context.Background(), fixedStatus{}, true)
	if err == nil || !strings.Contains(err.Error(), "authenticated state was not observed") {
		t.Fatalf("err = %v", err)
	}
	status, err := Poll{}.AuthState(context.Background(), fixedStatus{}, false)
	if err != nil || status.Authenticated {
		t.Fatalf("already unauthenticated: %#v, %v", status, err)
	}
}

func TestPollRowsRetriesUntilDone(t *testing.T) {
	calls := 0
	list := func(context.Context) ([]Row, error) {
		calls++
		if calls < 3 {
			return []Row{{"id": "synthetic"}}, nil
		}
		return []Row{}, nil
	}
	poll := Poll{Timeout: time.Second, Interval: time.Millisecond}
	if err := poll.Rows(context.Background(), list, IDAbsent("synthetic"), "synthetic timeout"); err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d", calls)
	}
	if err := (Poll{}).Rows(context.Background(), list, DevicePresent("absent"), "synthetic timeout"); err == nil || err.Error() != "synthetic timeout" {
		t.Fatalf("timeout err = %v", err)
	}
}

func TestRowPredicatesMatchNumericAndPaddedValues(t *testing.T) {
	rows := []Row{{"id": json.Number("42"), "device": " synthetic-device "}}
	if IDAbsent("42")(rows) {
		t.Fatal("numeric id was not matched")
	}
	if !DevicePresent("synthetic-device")(rows) {
		t.Fatal("padded device name was not matched")
	}
}
