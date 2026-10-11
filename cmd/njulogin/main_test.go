package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appkit "github.com/soundadam/appkit/go"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = writer
	callErr := fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	os.Stdout = original
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close: %v", err)
	}
	return string(data), callErr
}

func requireUsageError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected usage error")
	}
	var syntax usageError
	if !errors.As(err, &syntax) {
		t.Fatalf("error type = %T, want usageError: %v", err, err)
	}
}

func withStdin(t *testing.T, input string, fn func() error) error {
	t.Helper()
	original := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := io.WriteString(writer, input); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = original
		_ = reader.Close()
	}()
	return fn()
}

func installFakeSSH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "args.log")
	relay := filepath.Join(dir, "relay.py")
	relaySource := `import socket
import sys
import threading

host, port = sys.argv[1].rsplit(":", 1)
sock = socket.create_connection((host.strip("[]"), int(port)), timeout=5)

def upload():
    try:
        while True:
            data = __import__("os").read(sys.stdin.fileno(), 65536)
            if not data:
                break
            sock.sendall(data)
    finally:
        try:
            sock.shutdown(socket.SHUT_WR)
        except OSError:
            pass

thread = threading.Thread(target=upload, daemon=True)
thread.start()
while True:
    data = sock.recv(65536)
    if not data:
        break
    sys.stdout.buffer.write(data)
    sys.stdout.buffer.flush()
sock.close()
thread.join(timeout=1)
`
	if err := os.WriteFile(relay, []byte(relaySource), 0o600); err != nil {
		t.Fatalf("WriteFile relay: %v", err)
	}
	ssh := filepath.Join(dir, "ssh")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$@" >> "$SSH_ARGS_LOG"
target=
previous=
for argument in "$@"; do
  if [ "$previous" = "-W" ]; then
    target=$argument
    break
  fi
  previous=$argument
done
if [ -z "$target" ]; then
  echo 'missing -W target' >&2
  exit 64
fi
exec python3 "$SSH_RELAY_SCRIPT" "$target"
`
	if err := os.WriteFile(ssh, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile ssh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSH_ARGS_LOG", argsLog)
	t.Setenv("SSH_RELAY_SCRIPT", relay)
	return argsLog
}

func TestParseLoginFields(t *testing.T) {
	document := `<form>
<input name="lt" value="lt-value">
<input value="submit" name="_eventId">
<input name="execution" value="e1s1">
<input name="dllt" value="userNamePasswordLogin">
<input name="rmShown" value="1">
</form>`
	fields := parseLoginFields(document)
	if fields.LT != "lt-value" || fields.EventID != "submit" || fields.Execution != "e1s1" || fields.DLLT != "userNamePasswordLogin" || fields.RMShown != "1" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestMaskingKeepsReportsRedacted(t *testing.T) {
	if got := maskAccount("123456789"); got != "123***789" {
		t.Fatalf("maskAccount = %q", got)
	}
	if got := maskText("测试用户"); got != "测**户" {
		t.Fatalf("maskText = %q", got)
	}
	if got := maskMAC("02:00:00:00:00:01"); got != "02:00:xx:xx:xx:01" {
		t.Fatalf("maskMAC = %q", got)
	}
}

func TestSaveAndLoadCredentialConfig(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	if err := saveCredentialConfig(credentialConfig{Username: "123456789", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	config, err := loadCredentialConfig()
	if err != nil {
		t.Fatalf("loadCredentialConfig: %v", err)
	}
	if config == nil {
		t.Fatal("loadCredentialConfig returned nil")
	}
	if config.Username != "123456789" {
		t.Fatalf("Username = %q", config.Username)
	}
	if config.Password != "synthetic-secret" {
		t.Fatalf("Password did not round-trip")
	}
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("config file mode = %o", mode)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat config directory: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("config directory mode = %o", mode)
	}
}

func TestSavedCredentialPreservesPasswordWhitespace(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	const password = "  synthetic secret  "
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: password}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	config, err := loadCredentialConfig()
	if err != nil {
		t.Fatalf("loadCredentialConfig: %v", err)
	}
	if config.Password != password {
		t.Fatalf("password changed during persistence: %q", config.Password)
	}
}

func TestLoadCredentialConfigRejectsUnknownAndTrailingFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NJULOGIN_CONFIG_DIR", dir)
	path := filepath.Join(dir, "config.json")
	for _, data := range []string{
		`{"username":"synthetic-user","password":"synthetic-secret","keyring_service":"legacy"}`,
		`{"username":"synthetic-user","password":"synthetic-secret"} {}`,
	} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := loadCredentialConfig(); err == nil {
			t.Fatalf("accepted invalid credential data: %s", data)
		}
	}
}

func TestSaveCredentialConfigRepairsExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NJULOGIN_CONFIG_DIR", dir)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod directory: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat file: %v", err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0o600 {
		t.Fatalf("config file mode = %o", mode)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat directory: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Fatalf("config directory mode = %o", mode)
	}
}

func TestLoadCredentialConfigRejectsPermissiveFileAndSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NJULOGIN_CONFIG_DIR", dir)
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod file: %v", err)
	}
	if _, err := loadCredentialConfig(); err == nil {
		t.Fatal("expected permissive config mode error")
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove config: %v", err)
	}
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("{\"username\":\"synthetic-user\"}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if _, err := loadCredentialConfig(); err == nil {
		t.Fatal("expected config symlink error")
	}
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "synthetic-secret"}); err == nil {
		t.Fatal("expected save to reject config symlink")
	}
}

func TestConfigFilePathUsesOverrideDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NJULOGIN_CONFIG_DIR", dir)
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	want := filepath.Join(dir, "config.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

// useSyntheticUserConfigDir points os.UserConfigDir at a temporary directory
// and returns njulogin's default configuration directory inside it.
func useSyntheticUserConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("NJULOGIN_CONFIG_DIR", "")
	dir, err := defaultConfigDir()
	if err != nil {
		t.Fatalf("defaultConfigDir: %v", err)
	}
	return dir
}

func dirMode(t *testing.T, dir string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("Lstat %s: %v", dir, err)
	}
	return info.Mode().Perm()
}

func TestUpdateStateSharesPrivateDefaultConfigDir(t *testing.T) {
	dir := useSyntheticUserConfigDir(t)
	createDefaultConfigDir()
	appkit.LoadAutoCheck(application()).SetEnabled(true)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("appkit state not written to %s: entries=%v err=%v", dir, entries, err)
	}
	if mode := dirMode(t, dir); mode != 0o700 {
		t.Fatalf("directory mode after update state write = %o, want 0700", mode)
	}
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "synthetic-secret"}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	appkit.LoadAutoCheck(application()).SetEnabled(true)
	config, err := loadCredentialConfig()
	if err != nil || config == nil {
		t.Fatalf("loadCredentialConfig beside update state: config=%v err=%v", config, err)
	}
}

func TestCreateDefaultConfigDirLeavesExistingDirectory(t *testing.T) {
	dir := useSyntheticUserConfigDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	createDefaultConfigDir()
	if mode := dirMode(t, dir); mode != 0o755 {
		t.Fatalf("existing directory mode = %o, want it left at 0755", mode)
	}
}

func TestPortalSessionRowsAuthenticated(t *testing.T) {
	sessions := []sessionRow{{Username: "123456789"}}
	results, err := json.Marshal(getInfoResults{
		Rows: &sessions,
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rows, authenticated, err := portalSessionRows(&portalResponse{
		ReplyCode: 0,
		Results:   results,
	})
	if err != nil {
		t.Fatalf("portalSessionRows: %v", err)
	}
	if !authenticated {
		t.Fatal("authenticated = false")
	}
	if len(rows) != 1 || rows[0].Username != "123456789" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestPortalSessionRowsUnauthenticatedReplyCode(t *testing.T) {
	rows, authenticated, err := portalSessionRows(&portalResponse{
		ReplyCode: 404,
		Results:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("portalSessionRows: %v", err)
	}
	if authenticated {
		t.Fatal("authenticated = true")
	}
	if rows != nil {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestOnlineIDAtIndex(t *testing.T) {
	rows := []map[string]any{
		{"id": "a1"},
		{"id": 42},
	}
	id, err := onlineIDAtIndex(rows, 2)
	if err != nil {
		t.Fatalf("onlineIDAtIndex: %v", err)
	}
	if id != "42" {
		t.Fatalf("id = %q", id)
	}
}

func TestOnlineIDAtIndexRejectsOutOfRange(t *testing.T) {
	_, err := onlineIDAtIndex([]map[string]any{{"id": "a1"}}, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveMenuChoiceUsesDefaultOnBlank(t *testing.T) {
	choice, err := resolveMenuChoice("", "1", []menuOption{
		{Key: "1", Label: "local"},
		{Key: "2", Label: "ssh"},
	})
	if err != nil {
		t.Fatalf("resolveMenuChoice: %v", err)
	}
	if choice != "1" {
		t.Fatalf("choice = %q", choice)
	}
}

func TestResolveMenuChoiceRejectsUnknownValue(t *testing.T) {
	_, err := resolveMenuChoice("9", "1", []menuOption{
		{Key: "1", Label: "local"},
		{Key: "2", Label: "ssh"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoWithoutRedirectsAccepts302AndPreservesCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/submit":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/missing", http.StatusFound)
		case "/missing":
			http.NotFound(w, r)
		case "/check":
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
	client := &http.Client{Jar: jar}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/submit", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if _, err := doWithoutRedirects(req, client); err != nil {
		t.Fatalf("doWithoutRedirects: %v", err)
	}
	response, err := getPortalResponse(client, server.URL, "/check")
	if err != nil {
		t.Fatalf("getPortalResponse: %v", err)
	}
	rows, authenticated, err := portalSessionRows(response)
	if err != nil {
		t.Fatalf("portalSessionRows: %v", err)
	}
	if !authenticated || len(rows) != 1 || rows[0].Username != "123456789" {
		t.Fatalf("unexpected portal response: %#v authenticated=%v", rows, authenticated)
	}
}

func TestHTTPStatusErrorDoesNotExposeTokenBearingURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic upstream failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	const token = "synthetic-private-token"
	req, err := http.NewRequest(http.MethodGet, server.URL+"/status?uuid="+token, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = do(req, server.Client())
	if err == nil {
		t.Fatal("expected HTTP status error")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("HTTP error exposed URL or token: %v", err)
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("HTTP error omitted status: %v", err)
	}
}

func TestHTTPTransportErrorDoesNotExposeTokenBearingURL(t *testing.T) {
	const token = "synthetic-private-token"
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New(request.URL.String())
	})}
	req, err := http.NewRequest(http.MethodGet, "https://example.invalid/status?uuid="+token, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	_, err = do(req, client)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "example.invalid") {
		t.Fatalf("transport error exposed URL or token: %v", err)
	}
	if err.Error() != "HTTP request failed" {
		t.Fatalf("transport error = %q", err)
	}
}

func TestReadResponseBodyRejectsOversizedPayload(t *testing.T) {
	payload := strings.Repeat("x", maxResponseBodyBytes+1)
	if _, err := readResponseBody(strings.NewReader(payload)); err == nil {
		t.Fatal("expected oversized response error")
	}
	allowed := strings.Repeat("x", maxResponseBodyBytes)
	data, err := readResponseBody(strings.NewReader(allowed))
	if err != nil {
		t.Fatalf("readResponseBody at limit: %v", err)
	}
	if len(data) != maxResponseBodyBytes {
		t.Fatalf("response length = %d", len(data))
	}
}

func TestVersionIsDefined(t *testing.T) {
	if version == "" {
		t.Fatal("version must not be empty")
	}
}

func TestCredentialSourcesAreMutuallyExclusive(t *testing.T) {
	tests := []struct {
		name    string
		command string
		ask     bool
		stdin   bool
	}{
		{name: "command and ask", command: "printf synthetic", ask: true},
		{name: "command and stdin", command: "printf synthetic", stdin: true},
		{name: "ask and stdin", ask: true, stdin: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
			username := "synthetic-user"
			_, err := resolveLoginCredential(&username, test.command, test.ask, test.stdin)
			requireUsageError(t, err)
		})
	}
}

func TestSavedCredentialDefaultAndExplicitOverride(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "saved-secret"}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	username := ""
	secret, err := resolveLoginCredential(&username, "", false, false)
	if err != nil {
		t.Fatalf("resolve saved credential: %v", err)
	}
	if username != "synthetic-user" || secret != "saved-secret" {
		t.Fatalf("saved credential resolved username=%q secret=%q", username, secret)
	}
	username = "different-user"
	if _, err := resolveLoginCredential(&username, "", false, false); err == nil {
		t.Fatal("expected saved-username mismatch")
	}
	secret, err = resolveLoginCredential(&username, "printf explicit-secret", false, false)
	if err != nil || secret != "explicit-secret" {
		t.Fatalf("explicit override secret=%q err=%v", secret, err)
	}
}

func TestExplicitCredentialBypassesInvalidSavedConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NJULOGIN_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	username := "synthetic-user"
	secret, err := resolveLoginCredential(&username, "printf explicit-secret", false, false)
	if err != nil || secret != "explicit-secret" {
		t.Fatalf("explicit credential secret=%q err=%v", secret, err)
	}
}

func TestPasswordCommandIsBounded(t *testing.T) {
	original := passwordCommandTimeout
	passwordCommandTimeout = 20 * time.Millisecond
	t.Cleanup(func() { passwordCommandTimeout = original })

	started := time.Now()
	_, err := resolveExplicitSecret("sleep 1; printf synthetic-secret", false, false)
	if err == nil || err.Error() != "password command timed out" {
		t.Fatalf("password command error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("password command timeout took %s", elapsed)
	}
}

func TestRequireOperationSuccess(t *testing.T) {
	if err := requireOperationSuccess([]byte(`{"reply_code":0}`)); err != nil {
		t.Fatalf("success response: %v", err)
	}
	if err := requireOperationSuccess([]byte(`{"reply_code":7,"reply_msg":"synthetic rejection"}`)); err == nil {
		t.Fatal("expected portal rejection error")
	}
	if err := requireOperationSuccess([]byte(`not-json`)); err == nil {
		t.Fatal("expected malformed response error")
	}
	for _, data := range []string{
		`{}`,
		`{"reply_code":null}`,
		`{"reply_code":"0"}`,
	} {
		if err := requireOperationSuccess([]byte(data)); err == nil {
			t.Fatalf("expected missing or invalid reply_code error for %s", data)
		}
	}
}

func TestPrintStatusResponseRejectsMalformedResults(t *testing.T) {
	for _, results := range []string{
		`"not-an-object"`,
		`{}`,
		`{"rows":null}`,
	} {
		response := &portalResponse{ReplyCode: 0, Results: json.RawMessage(results)}
		if err := printStatusResponse(response); err == nil {
			t.Fatalf("expected malformed results error for %s", results)
		}
	}
	response := &portalResponse{ReplyCode: 0, Results: json.RawMessage(`{"rows":[]}`)}
	if err := printStatusResponse(response); err != nil {
		t.Fatalf("explicit empty rows must remain valid: %v", err)
	}
}

func TestListResponsesRequireExplicitRows(t *testing.T) {
	for _, results := range []string{`{}`, `{"rows":null}`} {
		response := &portalResponse{ReplyCode: 0, Results: json.RawMessage(results)}
		if _, err := quickRowsFromResponse(response); err == nil {
			t.Fatalf("quick response accepted missing rows: %s", results)
		}
		if _, err := onlineRowsFromResponse(response); err == nil {
			t.Fatalf("online response accepted missing rows: %s", results)
		}
	}

	empty := &portalResponse{ReplyCode: 0, Results: json.RawMessage(`{"rows":[]}`)}
	quick, err := quickRowsFromResponse(empty)
	if err != nil || len(quick) != 0 {
		t.Fatalf("explicit empty quick rows = %#v, %v", quick, err)
	}
	online, err := onlineRowsFromResponse(empty)
	if err != nil || len(online) != 0 {
		t.Fatalf("explicit empty online rows = %#v, %v", online, err)
	}
}

func TestStableNumberedRowKeys(t *testing.T) {
	output, err := captureStdout(t, func() error {
		if err := printOnlineRows([]map[string]any{{
			"id":            "synthetic-session",
			"acctstarttime": float64(1),
			"mac":           "02:00:00:00:00:01",
			"user_ipv6":     "2001:db8::1",
		}}); err != nil {
			return err
		}
		return printQuickRows([]map[string]any{{
			"id":     "synthetic-device",
			"mac":    "02:00:00:00:00:02",
			"device": "synthetic-device",
		}})
	})
	if err != nil {
		t.Fatalf("print rows: %v", err)
	}
	if !strings.Contains(output, "online_1=") {
		t.Fatalf("missing online_1 record: %q", output)
	}
	if !strings.Contains(output, "quick_1=") {
		t.Fatalf("missing quick_1 record: %q", output)
	}
	if strings.Contains(output, "online index=") || strings.Contains(output, "quick device_id=") {
		t.Fatalf("legacy row format remained: %q", output)
	}
}

func TestStableRecordsNormalizeControlCharacters(t *testing.T) {
	rows := []sessionRow{{
		Username:    "abc\n123456",
		Fullname:    "A\r\nB",
		ServiceName: "service\nstate=injected",
		MAC:         "02:00:00:00:00:01\nextra",
	}}
	results, err := json.Marshal(getInfoResults{Rows: &rows})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	output, err := captureStdout(t, func() error {
		if err := printStatusResponse(&portalResponse{ReplyCode: 0, Results: results}); err != nil {
			return err
		}
		return printQuickRows([]map[string]any{{
			"id":     "device\nresult=forged",
			"device": "name\twith\x1b[31mcontrol",
			"mac":    "02:00:00:00:00:02",
		}})
	})
	if err != nil {
		t.Fatalf("print records: %v", err)
	}
	if strings.ContainsAny(output, "\r\x1b") {
		t.Fatalf("stable output contains control characters: %q", output)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("stable output line count = %d: %q", len(lines), output)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "state=injected") || strings.HasPrefix(line, "result=forged") {
			t.Fatalf("field injected a stable record: %q", output)
		}
	}
}

func TestExitCodeClassification(t *testing.T) {
	if got := exitCodeForError(usagef("synthetic syntax")); got != 2 {
		t.Fatalf("usage exit code = %d", got)
	}
	if got := exitCodeForError(errors.New("synthetic runtime")); got != 1 {
		t.Fatalf("runtime exit code = %d", got)
	}
}

func TestPortalBaseURLValidation(t *testing.T) {
	for _, value := range []string{
		"https://p.nju.edu.cn",
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	} {
		if err := validatePortalBaseURL(value); err != nil {
			t.Fatalf("validatePortalBaseURL(%q): %v", value, err)
		}
	}
	requireUsageError(t, validatePortalBaseURL("http://portal.example.invalid"))
	requireUsageError(t, validatePortalBaseURL("https://user:secret@example.invalid"))
	requireUsageError(t, validatePortalBaseURL("https://example.invalid?token=synthetic"))
	requireUsageError(t, validatePortalBaseURL("https://example.invalid#fragment"))
}

func TestRemoteInputValidation(t *testing.T) {
	requireUsageError(t, validateSSHHost("-oProxyCommand=synthetic"))
	requireUsageError(t, validateSSHHost("synthetic host"))
	requireUsageError(t, validateSSHHost("synthetic\nhost"))
	requireUsageError(t, validateRemoteSettings(remoteSettings{
		BaseURL:        "https://p.nju.edu.cn",
		ConnectTimeout: 0,
		MaxTime:        15,
	}))
	requireUsageError(t, validateRemoteSettings(remoteSettings{
		BaseURL:        "https://p.nju.edu.cn",
		ConnectTimeout: 5,
		MaxTime:        0,
	}))
	requireUsageError(t, validateRemoteSettings(remoteSettings{
		BaseURL:        "https://p.nju.edu.cn",
		ConnectTimeout: maxConnectTimeout + 1,
		MaxTime:        15,
	}))
	requireUsageError(t, validateRemoteSettings(remoteSettings{
		BaseURL:        "https://p.nju.edu.cn",
		ConnectTimeout: 5,
		MaxTime:        maxRequestTimeout + 1,
	}))
}

func TestLoginCommandRejectsPortalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/portal/v1/login" {
			t.Fatalf("unexpected request after rejected login: %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"reply_code":1,"reply_msg":"synthetic rejection"}`)
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())

	err := loginCommand([]string{
		"--username", "synthetic-user",
		"--password-command", "printf synthetic-secret",
	})
	if err == nil {
		t.Fatal("expected rejected login error")
	}
}

func TestLoginCommandVerifiesAuthenticatedState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/portal/v1/login":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("Decode login body: %v", err)
			}
			if body["username"] != "synthetic-user" || body["password"] != "synthetic-secret" {
				t.Fatalf("unexpected login body: %#v", body)
			}
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user","fullname":"Synthetic User","mac":"02:00:00:00:00:01","service_name":"synthetic","acctstarttime":1}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())

	output, err := captureStdout(t, func() error {
		return loginCommand([]string{
			"--username", "synthetic-user",
			"--password-command", "printf synthetic-secret",
		})
	})
	if err != nil {
		t.Fatalf("loginCommand: %v", err)
	}
	if !strings.Contains(output, "state=authenticated") || !strings.Contains(output, "session_1=") {
		t.Fatalf("unexpected login output: %q", output)
	}
}

func TestLogoutCommandVerifiesUnauthenticatedState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/portal/v1/logout":
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)

	output, err := captureStdout(t, logoutCommand)
	if err != nil {
		t.Fatalf("logoutCommand: %v", err)
	}
	if !strings.Contains(output, "state=not_authenticated") {
		t.Fatalf("unexpected logout output: %q", output)
	}
}

func TestQuickAddAndDeleteVerifyBindings(t *testing.T) {
	present := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/portal/v1/quicklogin/bind/default":
			present = true
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case strings.HasPrefix(r.URL.Path, "/api/portal/v1/quicklogin/unbind/id/"):
			present = false
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/portal/v1/quicklogin/domain/default":
			if present {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-device","device":"synthetic-device","mac":"02:00:00:00:00:02"}]}}`)
			} else {
				_, _ = io.WriteString(w, `{"reply_code":404,"results":{"rows":[]}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)

	output, err := captureStdout(t, func() error {
		return quickCommand([]string{"add", "--name", "synthetic-device"})
	})
	if err != nil || !strings.Contains(output, "result=success") {
		t.Fatalf("quick add output=%q err=%v", output, err)
	}
	output, err = captureStdout(t, func() error {
		return quickCommand([]string{"delete", "--id", "synthetic-device"})
	})
	if err != nil || !strings.Contains(output, "result=success") {
		t.Fatalf("quick delete output=%q err=%v", output, err)
	}
}

func TestKickCommandVerifiesTargetAbsent(t *testing.T) {
	present := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/selfservice/v1/online/kick/"):
			present = false
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/selfservice/v1/online":
			if present {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-session"}]}}`)
			} else {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)

	output, err := captureStdout(t, func() error {
		return kickCommand([]string{"--id", "synthetic-session"})
	})
	if err != nil || !strings.Contains(output, "result=success") {
		t.Fatalf("kick output=%q err=%v", output, err)
	}
}

func TestAuthenticationWaitTimesOutWhenStateDoesNotChange(t *testing.T) {
	_, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return &portalResponse{ReplyCode: 0, Results: json.RawMessage(`{"rows":[]}`)}, nil
	}, true, 0)
	if err == nil {
		t.Fatal("expected authentication timeout")
	}
}

func TestSyntaxValidationAvoidsNetworkWork(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	requireUsageError(t, kickCommand([]string{"--id", "synthetic", "--index", "1"}))
	requireUsageError(t, qrCommand([]string{"--timeout", "0s"}))
	requireUsageError(t, run([]string{"status", "extra"}))
	requireUsageError(t, run([]string{"unknown-command"}))
	requireUsageError(t, run([]string{"kick", "--id", "synthetic", "--index", "1"}))
	requireUsageError(t, run([]string{"help", "extra"}))
	requireUsageError(t, run([]string{"version", "extra"}))
	requireUsageError(t, run([]string{"update", "extra"}))
	requireUsageError(t, run([]string{"update", "--unknown-flag"}))
	requireUsageError(t, run([]string{"update", "--check", "--auto", "off"}))
	requireUsageError(t, run([]string{"update", "--auto", "maybe"}))
}

func TestPostconditionPollingUsesBoundedInterval(t *testing.T) {
	original := postconditionPollInterval
	postconditionPollInterval = time.Millisecond
	t.Cleanup(func() { postconditionPollInterval = original })
	calls := 0
	err := waitForRows(func() ([]map[string]any, error) {
		calls++
		return []map[string]any{}, nil
	}, func([]map[string]any) bool {
		return calls >= 2
	}, 50*time.Millisecond, "synthetic timeout")
	if err != nil {
		t.Fatalf("waitForRows: %v", err)
	}
	if calls < 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestLocalStatusInfoAndRunDispatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user","fullname":"Synthetic User","mac":"02:00:00:00:00:01","service_name":"synthetic","acctstarttime":1}]}}`)
		case "/api/selfservice/v1/online":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-session","mac":"02:00:00:00:00:01","acctstarttime":1,"user_ipv6":"2001:db8::1"}]}}`)
		case "/api/portal/v1/quicklogin/domain/default":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-device","device":"synthetic-device","mac":"02:00:00:00:00:02"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)

	output, err := captureStdout(t, func() error { return run([]string{"status"}) })
	if err != nil {
		t.Fatalf("run status: %v", err)
	}
	if !strings.Contains(output, "state=authenticated") {
		t.Fatalf("status output: %q", output)
	}

	output, err = captureStdout(t, func() error { return run([]string{"info"}) })
	if err != nil {
		t.Fatalf("run info: %v", err)
	}
	for _, expected := range []string{"state=authenticated", "online_1=", "quick_1="} {
		if !strings.Contains(output, expected) {
			t.Fatalf("info output missing %q: %q", expected, output)
		}
	}

	output, err = captureStdout(t, func() error { return run([]string{"version"}) })
	if err != nil {
		t.Fatalf("run version: %v", err)
	}
	if output != "njulogin "+version+"\n" {
		t.Fatalf("version output: %q", output)
	}
	output, err = captureStdout(t, func() error { return run([]string{"--version"}) })
	if err != nil || output != "njulogin "+version+"\n" {
		t.Fatalf("run --version: %q, %v", output, err)
	}

	if err := run([]string{"help"}); err != nil {
		t.Fatalf("run help: %v", err)
	}
}

func TestLocalQRCommandCompletesAuthenticatedFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, `<form>
<input name="lt" value="synthetic-lt">
<input name="dllt" value="qrLogin">
<input name="execution" value="e1s1">
<input name="_eventId" value="submit">
<input name="rmShown" value="1">
</form>`)
				return
			}
			_, _ = io.WriteString(w, "submitted")
		case "/qrCode/getToken":
			_, _ = io.WriteString(w, "synthetic-token")
		case "/qrCode/getStatus.htl":
			_, _ = io.WriteString(w, "1")
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user","fullname":"Synthetic User","mac":"02:00:00:00:00:01","service_name":"synthetic","acctstarttime":1}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalBase := localCASBaseURL
	originalDest := localCASDestURL
	localCASBaseURL = server.URL
	localCASDestURL = server.URL + "/service"
	t.Cleanup(func() {
		localCASBaseURL = originalBase
		localCASDestURL = originalDest
	})
	t.Setenv("NJU_PORTAL_BASE_URL", server.URL)

	output, err := captureStdout(t, func() error {
		return qrCommand([]string{"--timeout", "1s"})
	})
	if err != nil {
		t.Fatalf("qrCommand: %v", err)
	}
	for _, expected := range []string{"scan_state=waiting", "scan_state=login_submitted", "state=authenticated", "session_1="} {
		if !strings.Contains(output, expected) {
			t.Fatalf("QR output missing %q", expected)
		}
	}
}

func TestRemoteCommandsUseLocalHTTPOverSSHTunnel(t *testing.T) {
	argsLog := installFakeSSH(t)
	authenticated := false
	quickPresent := false
	onlinePresent := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/portal/v1/login":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode login: %v", err)
			}
			if body["password"] != "synthetic-secret" {
				t.Fatalf("unexpected password body")
			}
			authenticated = true
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/portal/v1/logout":
			authenticated = false
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/portal/v1/getinfo":
			if authenticated {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user","fullname":"Synthetic User","mac":"02:00:00:00:00:01","service_name":"synthetic","acctstarttime":1}]}}`)
			} else {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
			}
		case r.URL.Path == "/api/portal/v1/quicklogin/bind/default":
			quickPresent = true
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case strings.HasPrefix(r.URL.Path, "/api/portal/v1/quicklogin/unbind/id/"):
			quickPresent = false
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/portal/v1/quicklogin/domain/default":
			if quickPresent {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-device","device":"synthetic-device","mac":"02:00:00:00:00:02"}]}}`)
			} else {
				_, _ = io.WriteString(w, `{"reply_code":404,"results":{"rows":[]}}`)
			}
		case strings.HasPrefix(r.URL.Path, "/api/selfservice/v1/online/kick/"):
			onlinePresent = false
			_, _ = io.WriteString(w, `{"reply_code":0}`)
		case r.URL.Path == "/api/selfservice/v1/online":
			if onlinePresent {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"id":"synthetic-session","mac":"02:00:00:00:00:01","acctstarttime":1}]}}`)
			} else {
				_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	settings := remoteSettings{BaseURL: server.URL, ConnectTimeout: 5, MaxTime: 15}
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())

	output, err := captureStdout(t, func() error {
		return remoteLoginCommand("synthetic-host", settings, []string{
			"--username", "synthetic-user",
			"--password-command", "printf synthetic-secret",
			"--add-quick", "synthetic-device",
		})
	})
	if err != nil || !strings.Contains(output, "state=authenticated") {
		t.Fatalf("remote login output=%q err=%v", output, err)
	}
	if _, err := captureStdout(t, func() error {
		return remoteQuickCommand("synthetic-host", settings, []string{"delete", "--id", "synthetic-device"})
	}); err != nil {
		t.Fatalf("remote quick delete: %v", err)
	}
	if _, err := captureStdout(t, func() error {
		return remoteKickCommand("synthetic-host", settings, []string{"--id", "synthetic-session"})
	}); err != nil {
		t.Fatalf("remote kick: %v", err)
	}
	if _, err := captureStdout(t, func() error {
		return remoteLogoutCommand("synthetic-host", settings)
	}); err != nil {
		t.Fatalf("remote logout: %v", err)
	}

	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile SSH args: %v", err)
	}
	text := string(args)
	if !strings.Contains(text, "-W\n") || strings.Contains(text, "curl") || strings.Contains(text, "python") || strings.Contains(text, "synthetic-secret") {
		t.Fatalf("unexpected SSH invocation surface: %q", text)
	}
}

func TestRemoteQRUsesLocalBootstrapOverSSHTunnel(t *testing.T) {
	argsLog := installFakeSSH(t)
	statusCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, `<input name="lt" value="lt"><input name="dllt" value="d"><input name="execution" value="e"><input name="_eventId" value="submit"><input name="rmShown" value="1">`)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/done", http.StatusFound)
		case "/qrCode/getToken":
			_, _ = io.WriteString(w, "synthetic-token")
		case "/qrCode/getStatus.htl":
			statusCalls++
			_, _ = io.WriteString(w, "1")
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[{"username":"synthetic-user"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	originalCAS, originalDest := localCASBaseURL, localCASDestURL
	localCASBaseURL, localCASDestURL = server.URL, server.URL+"/destination"
	t.Cleanup(func() { localCASBaseURL, localCASDestURL = originalCAS, originalDest })
	settings := remoteSettings{BaseURL: server.URL, ConnectTimeout: 5, MaxTime: 15}
	output, err := captureStdout(t, func() error { return remoteQRCommand("synthetic-host", settings) })
	if err != nil || statusCalls == 0 || !strings.Contains(output, "state=authenticated") {
		t.Fatalf("remote QR output=%q calls=%d err=%v", output, statusCalls, err)
	}
	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile SSH args: %v", err)
	}
	text := string(args)
	if strings.Contains(text, "python") || strings.Contains(text, "QR_URL") || strings.Contains(text, "synthetic-token") {
		t.Fatalf("remote QR leaked bootstrap material into SSH args: %q", text)
	}
}

func TestPromptHelpersRemainPlatformNeutral(t *testing.T) {
	if err := withStdin(t, "value\n", func() error {
		value, err := promptLine("prompt: ")
		if err != nil || value != "value" {
			t.Fatalf("promptLine value=%q err=%v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := runInteractiveLocal("unsupported"); err == nil {
		t.Fatal("expected unsupported local action")
	}
	if err := runInteractiveRemote("synthetic-host", "unsupported"); err == nil {
		t.Fatal("expected unsupported remote action")
	}
}

func TestDefaultRemoteSettingsUseNoRemoteProxyOrRuntime(t *testing.T) {
	t.Setenv("NJU_PORTAL_BASE_URL", "https://example.invalid")
	settings := defaultRemoteSettings()
	if settings.BaseURL != "https://example.invalid" || settings.Insecure || settings.ConnectTimeout != 5 || settings.MaxTime != 15 {
		t.Fatalf("defaultRemoteSettings = %#v", settings)
	}
}

func TestSetupCommandStoresLocalCredential(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	original := credentialPrompt
	credentialPrompt = func(prompt string) (string, error) {
		if prompt != "NJU credential: " {
			t.Fatalf("unexpected prompt: %q", prompt)
		}
		return "synthetic-secret", nil
	}
	t.Cleanup(func() { credentialPrompt = original })

	output, err := captureStdout(t, func() error {
		return setupCommand([]string{"--username", "synthetic-user"})
	})
	if err != nil {
		t.Fatalf("setupCommand: %v", err)
	}
	if !strings.Contains(output, "state=configured") || !strings.Contains(output, "credential_store=local_file") {
		t.Fatalf("setup output = %q", output)
	}
	config, err := loadCredentialConfig()
	if err != nil {
		t.Fatalf("loadCredentialConfig: %v", err)
	}
	if config.Username != "synthetic-user" || config.Password != "synthetic-secret" {
		t.Fatalf("stored config = %#v", config)
	}
}

func TestRemoteDispatcherRunsInfoOverSSHTunnel(t *testing.T) {
	argsLog := installFakeSSH(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/portal/v1/getinfo":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
		case "/api/selfservice/v1/online":
			_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
		case "/api/portal/v1/quicklogin/domain/default":
			_, _ = io.WriteString(w, `{"reply_code":404,"results":{"rows":[]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	output, err := captureStdout(t, func() error {
		return remoteCommand([]string{
			"--base-url", server.URL,
			"--connect-timeout", "5",
			"--max-time", "15",
			"synthetic-host", "info",
		})
	})
	if err != nil {
		t.Fatalf("remoteCommand info: %v", err)
	}
	for _, record := range []string{"state=not_authenticated", "online_devices=0", "quick_login_devices=0"} {
		if !strings.Contains(output, record) {
			t.Fatalf("remote info missing %q: %q", record, output)
		}
	}
	statusOutput, err := captureStdout(t, func() error {
		return remoteCommand([]string{"--base-url", server.URL, "synthetic-host", "status"})
	})
	if err != nil || !strings.Contains(statusOutput, "state=not_authenticated") {
		t.Fatalf("remote status output=%q err=%v", statusOutput, err)
	}
	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatalf("ReadFile SSH args: %v", err)
	}
	if !strings.Contains(string(args), "-W\n") {
		t.Fatalf("remote dispatcher did not use SSH -W: %q", args)
	}
}

func TestSSHTunnelConnLifecycle(t *testing.T) {
	installFakeSSH(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "synthetic-body")
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")
	conn, err := dialSSHTunnel(context.Background(), "synthetic-host", "tcp", address, 5)
	if err != nil {
		t.Fatalf("dialSSHTunnel: %v", err)
	}
	if conn.LocalAddr().Network() != "ssh" || conn.RemoteAddr().String() != address {
		t.Fatalf("unexpected tunnel addresses local=%v remote=%v", conn.LocalAddr(), conn.RemoteAddr())
	}
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if err := request.Write(conn); err != nil {
		t.Fatalf("Write request: %v", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("Read body: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("Close body: %v", err)
	}
	if string(body) != "synthetic-body" {
		t.Fatalf("unexpected response: %q", body)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestPromptHelpersAndInteractiveGuards(t *testing.T) {
	if err := withStdin(t, "required\n", func() error {
		value, err := promptRequired("required: ")
		if err != nil || value != "required" {
			t.Fatalf("promptRequired value=%q err=%v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := withStdin(t, "\n", func() error {
		value, err := promptMenuChoice("choice", []menuOption{{Key: "1", Label: "one"}}, "1")
		if err != nil || value != "1" {
			t.Fatalf("promptMenuChoice value=%q err=%v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := withStdin(t, "\n", func() error {
		value, err := promptQuickLoginName("synthetic-default")
		if err != nil || value != "synthetic-default" {
			t.Fatalf("promptQuickLoginName value=%q err=%v", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := withStdin(t, "", func() error {
		if err := interactiveCommand(); err == nil {
			t.Fatal("expected non-terminal interactive guard")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteHTTPSIsVerifiedLocallyOverSSHTunnel(t *testing.T) {
	installFakeSSH(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"reply_code":0,"results":{"rows":[]}}`)
	}))
	defer server.Close()

	settings := remoteSettings{BaseURL: server.URL, ConnectTimeout: 5, MaxTime: 15}
	if err := remoteStatusCommand("synthetic-host", settings); err == nil {
		t.Fatal("expected local TLS verification failure for the synthetic certificate")
	}
	settings.Insecure = true
	output, err := captureStdout(t, func() error {
		return remoteStatusCommand("synthetic-host", settings)
	})
	if err != nil || !strings.Contains(output, "state=not_authenticated") {
		t.Fatalf("insecure tunneled status output=%q err=%v", output, err)
	}
}

func TestHTTPClientsIgnoreAmbientProxySettings(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	client, err := newPortalClient("https://example.invalid")
	if err != nil {
		t.Fatalf("newPortalClient: %v", err)
	}
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("local transport inherited an ambient proxy: %#v", client.http.Transport)
	}
	settings := remoteSettings{BaseURL: "https://example.invalid", ConnectTimeout: 5, MaxTime: 15}
	remoteClient, err := newSSHHTTPClient("synthetic-host", settings)
	if err != nil {
		t.Fatalf("newSSHHTTPClient: %v", err)
	}
	remoteTransport, ok := remoteClient.Transport.(*http.Transport)
	if !ok || remoteTransport.Proxy != nil || !remoteTransport.DisableKeepAlives {
		t.Fatalf("unexpected SSH transport settings: %#v", remoteClient.Transport)
	}
}

func TestCredentialDirectorySymlinkIsRejected(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("Mkdir target: %v", err)
	}
	link := filepath.Join(base, "credential-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	t.Setenv("NJULOGIN_CONFIG_DIR", link)
	if err := saveCredentialConfig(credentialConfig{Username: "synthetic-user", Password: "synthetic-secret"}); err == nil {
		t.Fatal("expected save to reject a symlinked credential directory")
	}
	path := filepath.Join(target, "config.json")
	if err := os.WriteFile(path, []byte(`{"username":"synthetic-user","password":"synthetic-secret"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := loadCredentialConfig(); err == nil {
		t.Fatal("expected load to reject a symlinked credential directory")
	}
}
