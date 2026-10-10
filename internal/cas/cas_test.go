package cas

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseFields(t *testing.T) {
	fields := ParseFields(`<form>
<input name="lt" value="lt-value">
<input value="submit" name="_eventId">
<input name="execution" value="e1s1&amp;x">
<input name="dllt" value="userNamePasswordLogin">
<input name="rmShown" value="1">
</form>`)
	want := Fields{LT: "lt-value", DLLT: "userNamePasswordLogin", Execution: "e1s1&x", EventID: "submit", RMShown: "1"}
	if fields != want {
		t.Fatalf("fields = %#v", fields)
	}
}

type recordingRenderer struct{ payload string }

func (r *recordingRenderer) Render(w io.Writer, payload string) error {
	r.payload = payload
	_, err := io.WriteString(w, "[qr]\n")
	return err
}

func casServer(t *testing.T, form string, states ...string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, form)
				return
			}
			if err := r.ParseForm(); err != nil || r.PostForm.Get("uuid") != "synthetic-token" || r.PostForm.Get("cllt") != "qrLogin" {
				http.Error(w, "unexpected form", http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/done", http.StatusFound)
		case "/qrCode/getToken":
			_, _ = io.WriteString(w, "synthetic-token\n")
		case "/qrCode/getStatus.htl":
			state := states[0]
			if len(states) > 1 {
				states = states[1:]
			}
			_, _ = io.WriteString(w, state)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

const completeForm = `<input name="lt" value="lt"><input name="dllt" value="d"><input name="execution" value="e"><input name="_eventId" value="submit"><input name="rmShown" value="1">`

func newLogin(server *httptest.Server, out io.Writer, renderer Renderer) QRLogin {
	return QRLogin{
		HTTP:         server.Client(),
		Endpoints:    Endpoints{BaseURL: server.URL, Service: server.URL + "/service"},
		Renderer:     renderer,
		Out:          out,
		Timeout:      time.Second,
		PollInterval: time.Millisecond,
	}
}

func TestQRLoginReportsEachStateOnce(t *testing.T) {
	server := casServer(t, completeForm, "0", "2", "2", "1")
	var out strings.Builder
	renderer := &recordingRenderer{}
	if err := newLogin(server, &out, renderer).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := "[qr]\nscan_state=waiting\nscan_state=confirmed_on_phone_pending\nscan_state=login_submitted\n"
	if out.String() != want {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.HasSuffix(renderer.payload, "/qrCode/qrCodeLogin.do?uuid=synthetic-token") {
		t.Fatalf("payload = %q", renderer.payload)
	}
}

func TestQRLoginFailures(t *testing.T) {
	cases := []struct {
		name   string
		form   string
		states []string
		want   string
	}{
		{"missing fields", `<input name="lt" value="lt">`, []string{"1"}, "CAS login page is missing required form fields"},
		{"expired", completeForm, []string{"3"}, "QR code expired"},
		{"timeout", completeForm, []string{"0"}, "QR login timed out"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := casServer(t, c.form, c.states...)
			login := newLogin(server, io.Discard, &recordingRenderer{})
			login.Timeout = 20 * time.Millisecond
			if err := login.Run(context.Background()); err == nil || err.Error() != c.want {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestTerminalQRRendersBlocks(t *testing.T) {
	var out strings.Builder
	if err := (TerminalQR{}).Render(&out, "https://example.invalid/qr"); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out.String(), "example.invalid") || out.Len() == 0 {
		t.Fatal("rendered QR must be blocks, not the payload text")
	}
}
