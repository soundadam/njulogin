package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	appkit "github.com/soundadam/appkit/go"

	"github.com/soundadam/njulogin/internal/cas"
	"github.com/soundadam/njulogin/internal/credential"
	"github.com/soundadam/njulogin/internal/portal"
	"github.com/soundadam/njulogin/internal/testutil/fakessh"
	"github.com/soundadam/njulogin/internal/usage"
)

const authenticatedInfo = `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user","fullname":"Synthetic User","mac":"02:00:00:00:00:01","service_name":"synthetic","acctstarttime":1}]}}`

type testApp struct {
	*App
	out *bytes.Buffer
	err *bytes.Buffer
	env map[string]string
}

// newTestApp returns an App with a non-terminal stdin holding input, an empty
// credential directory, and fast postcondition polling.
func newTestApp(t *testing.T, input string) *testApp {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	if _, err := io.WriteString(writer, input); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	_ = writer.Close()
	t.Cleanup(func() { _ = reader.Close() })
	app := &testApp{
		out: &bytes.Buffer{},
		err: &bytes.Buffer{},
		env: map[string]string{"NJULOGIN_CONFIG_DIR": t.TempDir()},
	}
	app.App = New("dev")
	app.Stdin = reader
	app.Stdout = app.out
	app.Stderr = app.err
	app.Getenv = func(name string) string { return app.env[name] }
	app.Poll = portal.Poll{Timeout: time.Second, Interval: time.Millisecond}
	app.QRPollInterval = time.Millisecond
	return app
}

func (a *testApp) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	a.out.Reset()
	err := a.Run(context.Background(), args)
	return a.out.String(), err
}

func requireUsage(t *testing.T, err error) {
	t.Helper()
	if !usage.Is(err) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

// fakePortal is a synthetic portal whose state follows accepted operations.
type fakePortal struct {
	mu            sync.Mutex
	authenticated bool
	quickPresent  bool
	onlinePresent bool
	loginReply    string
	logins        []map[string]string
}

func (p *fakePortal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch path := r.URL.Path; {
	case path == "/api/portal/v1/login":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		p.logins = append(p.logins, body)
		if p.loginReply != "" {
			_, _ = io.WriteString(w, p.loginReply)
			return
		}
		p.authenticated = true
		_, _ = io.WriteString(w, `{"reply_code":0}`)
	case path == "/api/portal/v1/logout":
		p.authenticated = false
		_, _ = io.WriteString(w, `{"reply_code":0}`)
	case path == "/api/portal/v1/getinfo":
		if p.authenticated {
			_, _ = io.WriteString(w, authenticatedInfo)
		} else {
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
		}
	case path == "/api/portal/v1/quicklogin/bind/default":
		p.quickPresent = true
		_, _ = io.WriteString(w, `{"reply_code":0}`)
	case strings.HasPrefix(path, "/api/portal/v1/quicklogin/unbind/id/"):
		p.quickPresent = false
		_, _ = io.WriteString(w, `{"reply_code":0}`)
	case path == "/api/portal/v1/quicklogin/domain/default":
		if p.quickPresent {
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-device","device":"synthetic-device","mac":"02:00:00:00:00:02"}]}}`)
		} else {
			_, _ = io.WriteString(w, `{"reply_code":404,"results":{"rows":[]}}`)
		}
	case strings.HasPrefix(path, "/api/selfservice/v1/online/kick/"):
		p.onlinePresent = false
		_, _ = io.WriteString(w, `{"reply_code":0}`)
	case path == "/api/selfservice/v1/online":
		if p.onlinePresent {
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-session","mac":"02:00:00:00:00:01","acctstarttime":1,"user_ipv6":"2001:db8::1"}]}}`)
		} else {
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
		}
	default:
		http.NotFound(w, r)
	}
}

func startPortal(t *testing.T, app *testApp, p *fakePortal) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(p)
	t.Cleanup(server.Close)
	app.env["NJU_PORTAL_BASE_URL"] = server.URL
	return server
}

func TestLoginVerifiesAuthenticatedStateAndBindsQuick(t *testing.T) {
	app := newTestApp(t, "")
	p := &fakePortal{}
	startPortal(t, app, p)
	output, err := app.run(t, "login", "--username", "synthetic-user", "--password-command", "printf synthetic-secret", "--add-quick", "synthetic-device")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	for _, record := range []string{"state=authenticated\n", "sessions=1\n", "session_1=syn***ser S************r start=", "quick_add_result=success\n"} {
		if !strings.Contains(output, record) {
			t.Fatalf("login output missing %q: %q", record, output)
		}
	}
	if len(p.logins) != 1 || p.logins[0]["username"] != "synthetic-user" || p.logins[0]["password"] != "synthetic-secret" {
		t.Fatal("portal did not receive the resolved credential")
	}
}

func TestLoginRejectsPortalFailureWithoutPolling(t *testing.T) {
	app := newTestApp(t, "")
	p := &fakePortal{loginReply: `{"reply_code":1,"reply_msg":"synthetic rejection"}`}
	startPortal(t, app, p)
	_, err := app.run(t, "login", "--username", "synthetic-user", "--password-command", "printf synthetic-secret")
	if err == nil || err.Error() != "portal operation rejected with reply_code=1" || ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
}

func TestLoginReadsPasswordFromStdin(t *testing.T) {
	app := newTestApp(t, "synthetic-secret\n")
	p := &fakePortal{}
	startPortal(t, app, p)
	if _, err := app.run(t, "login", "--username", "synthetic-user", "--password-stdin"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if p.logins[0]["password"] != "synthetic-secret" {
		t.Fatal("stdin password was not used")
	}
}

func TestCredentialResolution(t *testing.T) {
	app := newTestApp(t, "")
	ctx := context.Background()
	for _, opts := range []loginOptions{
		{Username: "synthetic-user", PasswordCommand: "printf x", Ask: true},
		{Username: "synthetic-user", PasswordCommand: "printf x", Stdin: true},
		{Username: "synthetic-user", Ask: true, Stdin: true},
		{Username: "synthetic-user"},
		{},
	} {
		_, _, err := app.resolveCredential(ctx, opts)
		requireUsage(t, err)
	}

	store, err := app.store()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save(credential.Credential{Username: "synthetic-user", Password: "saved-secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	username, secret, err := app.resolveCredential(ctx, loginOptions{})
	if err != nil || username != "synthetic-user" || secret != "saved-secret" {
		t.Fatalf("saved credential: username=%q err=%v", username, err)
	}
	_, _, err = app.resolveCredential(ctx, loginOptions{Username: "different-user"})
	requireUsage(t, err)
	_, secret, err = app.resolveCredential(ctx, loginOptions{Username: "different-user", PasswordCommand: "printf explicit-secret"})
	if err != nil || secret != "explicit-secret" {
		t.Fatalf("explicit override err=%v", err)
	}

	if err := os.WriteFile(store.Path, []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, secret, err = app.resolveCredential(ctx, loginOptions{Username: "synthetic-user", PasswordCommand: "printf explicit-secret"}); err != nil || secret != "explicit-secret" {
		t.Fatalf("explicit source must bypass an invalid saved file: err=%v", err)
	}
	if _, _, err = app.resolveCredential(ctx, loginOptions{}); err == nil {
		t.Fatal("accepted an invalid saved file")
	}
}

func TestLogoutVerifiesUnauthenticatedState(t *testing.T) {
	app := newTestApp(t, "")
	startPortal(t, app, &fakePortal{authenticated: true})
	output, err := app.run(t, "logout")
	if err != nil || output != "state=not_authenticated\n" {
		t.Fatalf("logout output=%q err=%v", output, err)
	}
}

func TestQuickAddListAndDeleteVerifyBindings(t *testing.T) {
	app := newTestApp(t, "")
	startPortal(t, app, &fakePortal{})
	if output, err := app.run(t, "quick", "add", "--name", "synthetic-device"); err != nil || output != "result=success\n" {
		t.Fatalf("quick add output=%q err=%v", output, err)
	}
	if output, err := app.run(t, "quick", "list"); err != nil || output != "quick_login_devices=1\nquick_1=device_id=synthetic-device mac=02:00:xx:xx:xx:02 name=synthetic-device\n" {
		t.Fatalf("quick list output=%q err=%v", output, err)
	}
	if output, err := app.run(t, "quick", "delete", "--id", "synthetic-device"); err != nil || output != "result=success\n" {
		t.Fatalf("quick delete output=%q err=%v", output, err)
	}
}

func TestKickByIndexVerifiesTargetAbsent(t *testing.T) {
	app := newTestApp(t, "")
	startPortal(t, app, &fakePortal{onlinePresent: true})
	if output, err := app.run(t, "kick", "--index", "1"); err != nil || output != "result=success\n" {
		t.Fatalf("kick output=%q err=%v", output, err)
	}
}

func TestOnlineIDAtIndex(t *testing.T) {
	rows := []portal.Row{{"id": "a1"}, {"id": json.Number("42")}, {}}
	if id, err := onlineIDAtIndex(rows, 2); err != nil || id != "42" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	for _, index := range []int{0, 3, 4} {
		_, err := onlineIDAtIndex(rows, index)
		requireUsage(t, err)
	}
}

func TestStatusInfoVersionAndHelpDispatch(t *testing.T) {
	app := newTestApp(t, "")
	startPortal(t, app, &fakePortal{authenticated: true, onlinePresent: true, quickPresent: true})
	if output, err := app.run(t, "status"); err != nil || !strings.HasPrefix(output, "state=authenticated\nsessions=1\n") {
		t.Fatalf("status output=%q err=%v", output, err)
	}
	output, err := app.run(t, "info")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	sections := strings.Split(output, "\n\n")
	if len(sections) != 3 || !strings.HasPrefix(sections[1], "online_devices=1\nonline_1=start=") || !strings.HasSuffix(sections[1], "mac=02:00:xx:xx:xx:01 ipv4=redacted ipv6=present") || !strings.HasPrefix(sections[2], "quick_login_devices=1\n") {
		t.Fatalf("info output = %q", output)
	}
	if output, err := app.run(t, "version"); err != nil || !strings.HasPrefix(output, "njulogin ") {
		t.Fatalf("version output=%q err=%v", output, err)
	}
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"help", "login"}, {"login", "-h"}, {"remote", "h", "kick", "--help"}} {
		if output, err := app.run(t, args...); err != nil || !strings.Contains(output, "Usage:") {
			t.Fatalf("%q output=%q err=%v", args, output, err)
		}
	}
	if output, err := app.run(t, "completion", "zsh"); err != nil || !strings.HasPrefix(output, "#compdef njulogin") {
		t.Fatalf("completion output=%.40q err=%v", output, err)
	}
	if output, err := app.run(t, "--version"); err != nil || output != "njulogin dev\n" {
		t.Fatalf("--version output=%q err=%v", output, err)
	}
}

func TestSyntaxErrorsAvoidNetworkWork(t *testing.T) {
	app := newTestApp(t, "")
	app.env["NJU_PORTAL_BASE_URL"] = "https://example.invalid"
	for _, args := range [][]string{
		{},
		{"unknown-command"},
		{"status", "extra"},
		{"version", "extra"},
		{"kick"},
		{"kick", "--id", "synthetic", "--index", "1"},
		{"qr", "--timeout", "0s"},
		{"quick"},
		{"quick", "list", "extra"},
		{"quick", "add"},
		{"quick", "delete"},
		{"quick", "rename"},
		{"login", "positional"},
		{"setup", "--username", " "},
		{"remote", "synthetic-host"},
		{"login", "--unknown-flag"},
		{"kick", "--index", "one"},
		{"qr", "--timeout", "soon"},
		{"remote", "-oProxyCommand=x", "status"},
		{"remote", "--", "-oProxyCommand=x", "status"},
		{"remote", "synthetic-host", "login", "--password", "x"},
		{"remote", "synthetic-host", "qr", "--timeout", "1s"},
		{"remote", "--connect-timeout", "0", "synthetic-host", "status"},
		{"remote", "--max-time", "3601", "synthetic-host", "status"},
		{"remote", "--base-url", "http://example.invalid", "synthetic-host", "status"},
		{"remote", "synthetic-host", "status", "extra"},
		{"remote", "synthetic-host", "unknown"},
		{"remote", "synthetic-host", "quick"},
		{"update", "extra"},
		{"update", "--unknown-flag"},
		{"update", "--check", "--auto", "off"},
		{"update", "--auto", "maybe"},
	} {
		_, err := app.run(t, args...)
		if !usage.Is(err) || ExitCode(err) != 2 {
			t.Errorf("%q: err = %v, want usage error", args, err)
		}
	}
}

func TestRemoteUsageNamesTheRemoteForm(t *testing.T) {
	app := newTestApp(t, "")
	_, err := app.run(t, "remote", "synthetic-host", "quick")
	if err == nil || err.Error() != "usage: njulogin remote <host> quick [list|add|delete]" {
		t.Fatalf("err = %v", err)
	}
	_, err = app.run(t, "quick", "list", "extra")
	if err == nil || err.Error() != "quick list does not accept arguments" {
		t.Fatalf("err = %v", err)
	}
}

func casPortal(t *testing.T, statusCalls *int) *httptest.Server {
	t.Helper()
	p := &fakePortal{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, `<input name="lt" value="lt"><input name="dllt" value="d"><input name="execution" value="e"><input name="_eventId" value="submit"><input name="rmShown" value="1">`)
				return
			}
			p.mu.Lock()
			p.authenticated = true
			p.mu.Unlock()
			http.Redirect(w, r, "/done", http.StatusFound)
		case "/qrCode/getToken":
			_, _ = io.WriteString(w, "synthetic-token")
		case "/qrCode/getStatus.htl":
			*statusCalls++
			_, _ = io.WriteString(w, "1")
		default:
			p.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestLocalQRCompletesAuthenticatedFlow(t *testing.T) {
	app := newTestApp(t, "")
	calls := 0
	server := casPortal(t, &calls)
	app.env["NJU_PORTAL_BASE_URL"] = server.URL
	app.CAS = cas.Endpoints{BaseURL: server.URL, Service: server.URL + "/service"}
	output, err := app.run(t, "qr", "--timeout", "1s")
	if err != nil {
		t.Fatalf("qr: %v", err)
	}
	for _, record := range []string{"scan_state=waiting\n", "scan_state=login_submitted\nstate=authenticated\n", "session_1="} {
		if !strings.Contains(output, record) {
			t.Fatalf("QR output missing %q", record)
		}
	}
	if strings.Contains(output, "synthetic-token") {
		t.Fatal("QR output exposed the token")
	}
}

func TestRemoteCommandsRunOverSSHTunnel(t *testing.T) {
	argsLog := fakessh.Install(t)
	app := newTestApp(t, "")
	p := &fakePortal{onlinePresent: true}
	server := httptest.NewServer(p)
	defer server.Close()
	remote := func(args ...string) (string, error) {
		return app.run(t, append([]string{"remote", "--base-url", server.URL, "--connect-timeout", "5", "--max-time", "15", "synthetic-host"}, args...)...)
	}

	if output, err := remote("login", "--username", "synthetic-user", "--password-command", "printf synthetic-secret", "--add-quick", "synthetic-device"); err != nil || !strings.Contains(output, "quick_add_result=success") {
		t.Fatalf("remote login output=%q err=%v", output, err)
	}
	if output, err := remote("info"); err != nil || !strings.Contains(output, "online_devices=1") || !strings.Contains(output, "quick_login_devices=1") {
		t.Fatalf("remote info output=%q err=%v", output, err)
	}
	for _, args := range [][]string{
		{"quick", "list"},
		{"quick", "delete", "--id", "synthetic-device"},
		{"kick", "--id", "synthetic-session"},
		{"logout"},
	} {
		if _, err := remote(args...); err != nil {
			t.Fatalf("remote %q: %v", args, err)
		}
	}
	if output, err := remote("status"); err != nil || output != "state=not_authenticated\n" {
		t.Fatalf("remote status output=%q err=%v", output, err)
	}

	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(args)
	if !strings.Contains(text, "-W\n") || strings.Contains(text, "curl") || strings.Contains(text, "python") || strings.Contains(text, "synthetic-secret") {
		t.Fatalf("unexpected SSH invocation surface: %q", text)
	}
}

func TestRemoteQRBootstrapsLocally(t *testing.T) {
	argsLog := fakessh.Install(t)
	app := newTestApp(t, "")
	calls := 0
	server := casPortal(t, &calls)
	app.CAS = cas.Endpoints{BaseURL: server.URL, Service: server.URL + "/destination"}
	output, err := app.run(t, "remote", "--base-url", server.URL, "synthetic-host", "qr")
	if err != nil || calls == 0 || !strings.Contains(output, "state=authenticated") {
		t.Fatalf("remote QR output=%q calls=%d err=%v", output, calls, err)
	}
	args, _ := os.ReadFile(argsLog)
	if strings.Contains(string(args), "python") || strings.Contains(string(args), "synthetic-token") {
		t.Fatalf("remote QR leaked bootstrap material into SSH args: %q", args)
	}
}

func TestRemoteHTTPSIsVerifiedLocally(t *testing.T) {
	fakessh.Install(t)
	app := newTestApp(t, "")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
	}))
	defer server.Close()
	if _, err := app.run(t, "remote", "--base-url", server.URL, "synthetic-host", "status"); err == nil {
		t.Fatal("accepted an unverified certificate")
	}
	output, err := app.run(t, "remote", "--insecure", "--base-url", server.URL, "synthetic-host", "status")
	if err != nil || output != "state=not_authenticated\n" {
		t.Fatalf("insecure output=%q err=%v", output, err)
	}
	if !strings.Contains(app.err.String(), "warning: TLS verification is disabled") {
		t.Fatalf("missing insecure warning: %q", app.err.String())
	}
}

func TestDefaultRemoteSettingsUseNoRemoteProxyOrRuntime(t *testing.T) {
	settings := defaultRemoteSettings("https://example.invalid")
	if settings != (RemoteSettings{BaseURL: "https://example.invalid", ConnectTimeout: 5, MaxTime: 15}) {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestSetupStoresLocalCredential(t *testing.T) {
	app := newTestApp(t, "synthetic-user\n")
	app.SetupSecret = fixedSecret("synthetic-secret")
	output, err := app.run(t, "setup")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !strings.HasPrefix(output, "state=configured\naccount=syn***ser\ncredential_store=local_file\nconfig_path=") {
		t.Fatalf("setup output = %q", output)
	}
	store, _ := app.store()
	saved, err := store.Load()
	if err != nil || saved.Username != "synthetic-user" || saved.Password != "synthetic-secret" {
		t.Fatalf("saved = %#v, err = %v", saved, err)
	}
	app.SetupSecret = fixedSecret(" ")
	_, err = app.run(t, "setup", "--username", "synthetic-user")
	requireUsage(t, err)
}

func TestReportsNormalizeControlCharacters(t *testing.T) {
	var out bytes.Buffer
	writeStatus(&out, portal.Status{Authenticated: true, Sessions: []portal.Session{{
		Username:    "abc\n123456",
		Fullname:    "A\r\nB",
		ServiceName: "service\nstate=injected",
		MAC:         "02:00:00:00:00:01\nextra",
	}}})
	writeQuickRows(&out, []portal.Row{{
		"id":     "device\nresult=forged",
		"device": "name\twith\x1b[31mcontrol",
		"mac":    "02:00:00:00:00:02",
	}})
	output := out.String()
	if strings.ContainsAny(output, "\r\x1b\t") {
		t.Fatalf("output contains control characters: %q", output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("line count = %d: %q", len(lines), output)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "state=injected") || strings.HasPrefix(line, "result=forged") {
			t.Fatalf("field injected a record: %q", output)
		}
	}
}

func TestPromptLineReadsSharedInput(t *testing.T) {
	app := newTestApp(t, "first\nsecond\n")
	for _, want := range []string{"first", "second"} {
		if value, err := app.promptLine("prompt: "); err != nil || value != want {
			t.Fatalf("promptLine value=%q err=%v, want %q", value, err, want)
		}
	}
	if app.err.String() != "prompt: prompt: " {
		t.Fatalf("prompts = %q", app.err.String())
	}
}

func TestInteractiveBackendRunsDocumentedOperations(t *testing.T) {
	app := newTestApp(t, "")
	p := &fakePortal{onlinePresent: true}
	startPortal(t, app, p)
	ctx := context.Background()

	if _, err := app.connect("-oProxyCommand=x"); !usage.Is(err) {
		t.Fatalf("connect accepted an option-shaped host: %v", err)
	}
	b, err := app.connect("")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if b.HasSavedCredential() {
		t.Fatal("empty credential directory reported a saved credential")
	}
	var out bytes.Buffer
	if err := b.Login(ctx, &out, "", ""); !usage.Is(err) {
		t.Fatalf("login without any credential: %v", err)
	}
	if err := b.Setup(ctx, &out, "synthetic-user", "synthetic-secret"); err != nil || !b.HasSavedCredential() {
		t.Fatalf("setup err=%v", err)
	}
	out.Reset()
	if err := b.Login(ctx, &out, "", ""); err != nil || !strings.HasPrefix(out.String(), "state=authenticated") {
		t.Fatalf("saved login output=%q err=%v", out.String(), err)
	}
	if err := b.Login(ctx, &out, "synthetic-user", "typed-secret"); err != nil || p.logins[len(p.logins)-1]["password"] != "typed-secret" {
		t.Fatalf("typed login err=%v", err)
	}

	snapshot := b.Snapshot(ctx)
	if snapshot.StatusErr != nil || !snapshot.Status.Authenticated || len(snapshot.Online) != 1 || len(snapshot.Quick) != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	for _, step := range []func() error{
		func() error { return b.QuickAdd(ctx, &out, "synthetic-device") },
		func() error { return b.QuickDelete(ctx, &out, "synthetic-device") },
		func() error { return b.Kick(ctx, &out, 1) },
		func() error { return b.Logout(ctx, &out) },
	} {
		out.Reset()
		if err := step(); err != nil {
			t.Fatalf("operation output=%q err=%v", out.String(), err)
		}
	}
	if app.out.Len() != 0 {
		t.Fatalf("interactive operations wrote to standard output: %q", app.out.String())
	}

	remote, err := app.connect("synthetic-host")
	if err != nil || remote.Name() != "ssh synthetic-host" || remote.DeviceName() != "synthetic-host" {
		t.Fatalf("remote backend = %v, %v", remote, err)
	}
}

func TestResolveVersionPrefersLinkerThenModuleVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}
	cases := []struct {
		linked string
		info   *debug.BuildInfo
		want   string
	}{
		{"0.2.0", info, "0.2.0"},
		{"dev", info, "v0.1.0"},
		{"dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"", nil, "dev"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.linked, c.info); got != c.want {
			t.Errorf("resolveVersion(%q) = %q, want %q", c.linked, got, c.want)
		}
	}
}

// useSyntheticUserConfigDir points os.UserConfigDir at a temporary directory
// and returns njulogin's default configuration directory inside it.
func useSyntheticUserConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dir, err := credential.DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	return dir
}

func TestUpdateStateSharesPrivateDefaultConfigDir(t *testing.T) {
	dir := useSyntheticUserConfigDir(t)
	app := newTestApp(t, "")
	credential.CreateDefaultDir()
	appkit.LoadAutoCheck(app.identity()).SetEnabled(true)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("appkit state not written to %s: entries=%v err=%v", dir, entries, err)
	}
	if info, err := os.Lstat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("directory after update state write = %v, %v; want mode 0700", info, err)
	}
	path, err := credential.DefaultPath("")
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	store := credential.FileStore{Path: path}
	if err := store.Save(credential.Credential{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	appkit.LoadAutoCheck(app.identity()).SetEnabled(true)
	if saved, err := store.Load(); err != nil || saved == nil {
		t.Fatalf("Load beside update state: %v, %v", saved, err)
	}
}

func TestUpdateAutoTogglesAutomaticChecks(t *testing.T) {
	useSyntheticUserConfigDir(t)
	app := newTestApp(t, "")
	for _, value := range []string{"off", "on"} {
		if output, err := app.run(t, "update", "--auto", value); err != nil || output != "Automatic update checks are "+value+".\n" {
			t.Fatalf("update --auto %s: %q, %v", value, output, err)
		}
		if got := appkit.LoadAutoCheck(app.identity()).Enabled(); got != (value == "on") {
			t.Fatalf("after --auto %s, enabled = %v", value, got)
		}
	}
}
