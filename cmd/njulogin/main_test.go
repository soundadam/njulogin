package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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

func TestBuildRemoteCurlCommandUsesPortableQuotedArgs(t *testing.T) {
	settings := remoteSettings{
		BaseURL:        "https://p.nju.edu.cn",
		Proxy:          "",
		Insecure:       false,
		ConnectTimeout: 5,
		MaxTime:        15,
	}
	got := buildRemoteCurlCommand(settings, "POST", "/api/portal/v1/login", true)
	want := "'curl' '-sS' '--connect-timeout' '5' '--max-time' '15' '--noproxy' '*' '-X' 'POST' '-H' 'Content-Type: application/json' '--data' '@-' 'https://p.nju.edu.cn/api/portal/v1/login'"
	if got != want {
		t.Fatalf("buildRemoteCurlCommand = %q, want %q", got, want)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	if got := shellQuote("o'hare"); got != "'o'\"'\"'hare'" {
		t.Fatalf("shellQuote = %q", got)
	}
}

func TestSaveAndLoadCredentialConfig(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	if err := saveCredentialConfig(credentialConfig{
		Username:        "123456789",
		KeychainService: "",
	}); err != nil {
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
	if config.KeychainService != defaultKeychainService {
		t.Fatalf("KeychainService = %q", config.KeychainService)
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
}

func TestApplyCredentialDefaultsUsesSavedConfig(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	if err := saveCredentialConfig(credentialConfig{
		Username:        "123456789",
		KeychainService: "NJUlogin",
	}); err != nil {
		t.Fatalf("saveCredentialConfig: %v", err)
	}
	username := ""
	service := ""
	if err := applyCredentialDefaults(&username, &service, "", false, false); err != nil {
		t.Fatalf("applyCredentialDefaults: %v", err)
	}
	if username != "123456789" {
		t.Fatalf("username = %q", username)
	}
	if service != "NJUlogin" {
		t.Fatalf("service = %q", service)
	}
}

func TestApplyCredentialDefaultsUsesDefaultServiceWithoutConfig(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	username := "123456789"
	service := ""
	if err := applyCredentialDefaults(&username, &service, "", false, false); err != nil {
		t.Fatalf("applyCredentialDefaults: %v", err)
	}
	if service != defaultKeychainService {
		t.Fatalf("service = %q", service)
	}
}

func TestApplyCredentialDefaultsDoesNotForceKeychainDuringAsk(t *testing.T) {
	t.Setenv("NJULOGIN_CONFIG_DIR", t.TempDir())
	username := "123456789"
	service := ""
	if err := applyCredentialDefaults(&username, &service, "", true, false); err != nil {
		t.Fatalf("applyCredentialDefaults: %v", err)
	}
	if service != "" {
		t.Fatalf("service = %q", service)
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

func TestPortalSessionRowsAuthenticated(t *testing.T) {
	results, err := json.Marshal(getInfoResults{
		Rows: []sessionRow{{Username: "123456789"}},
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
		ReplyMsg:  "无此记录",
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

func TestVersionIsDefined(t *testing.T) {
	if version == "" {
		t.Fatal("version must not be empty")
	}
}

func TestKeychainStoreCommandKeepsPasswordOutOfArguments(t *testing.T) {
	const secret = "synthetic-test-secret"
	cmd := keychainStoreCommand("NJUlogin", "123456789", secret)
	for _, arg := range cmd.Args {
		if arg == secret {
			t.Fatal("password was included in process arguments")
		}
	}
	if len(cmd.Args) == 0 || cmd.Args[len(cmd.Args)-1] != "-w" {
		t.Fatalf("-w must be the final argument so security reads the password from stdin: %#v", cmd.Args)
	}
	if cmd.Stdin == nil {
		t.Fatal("password stdin is not configured")
	}
}
