package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/term"
)

const (
	portalBaseURL          = "https://p.nju.edu.cn"
	casBaseURL             = "https://authserver.nju.edu.cn/authserver"
	casDestURL             = "https://p.nju.edu.cn:443/api/cas/getinfo/&renew=true"
	defaultKeychainService = "NJUlogin"
)

var userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
var version = "dev"

//go:embed remote_qr_bootstrap.py
var remoteQRBootstrap string

type portalClient struct {
	baseURL string
	http    *http.Client
}

type portalResponse struct {
	ReplyCode int             `json:"reply_code"`
	ReplyMsg  string          `json:"reply_msg"`
	Results   json.RawMessage `json:"results"`
}

type getInfoResults struct {
	Rows []sessionRow `json:"rows"`
}

type sessionRow struct {
	Username      string `json:"username"`
	Fullname      string `json:"fullname"`
	MAC           string `json:"mac"`
	UserIPv6      string `json:"user_ipv6"`
	ServiceName   string `json:"service_name"`
	AcctStartTime int64  `json:"acctstarttime"`
}

type listResults struct {
	Rows []map[string]any `json:"rows"`
}

type loginFields struct {
	LT        string
	DLLT      string
	Execution string
	EventID   string
	RMShown   string
}

type remoteSettings struct {
	BaseURL        string
	Proxy          string
	Insecure       bool
	ConnectTimeout int
	MaxTime        int
}

type credentialConfig struct {
	Username        string `json:"username"`
	KeychainService string `json:"keychain_service"`
}

type menuOption struct {
	Key   string
	Label string
}

func newPortalClient(baseURL string) (*portalClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &portalClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}, nil
}

func (c *portalClient) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return do(req, c.http)
}

func (c *portalClient) postJSON(path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	return do(req, c.http)
}

func do(req *http.Request, client *http.Client) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s returned HTTP %d", req.URL.String(), resp.StatusCode)
	}
	return data, nil
}

func doWithoutRedirects(req *http.Request, client *http.Client) ([]byte, error) {
	clone := *client
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := clone.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return nil, fmt.Errorf("%s returned HTTP %d", req.URL.String(), resp.StatusCode)
	}
	return data, nil
}

func getJSON(path string) (*portalResponse, error) {
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return nil, err
	}
	return getPortalResponse(client.http, client.baseURL, path)
}

func parsePortalResponse(data []byte) (*portalResponse, error) {
	var response portalResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func getPortalResponse(client *http.Client, baseURL, path string) (*portalResponse, error) {
	portal := &portalClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    client,
	}
	data, err := portal.get(path)
	if err != nil {
		return nil, err
	}
	return parsePortalResponse(data)
}

func statusCommand() error {
	response, err := getJSON(fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	return printStatusResponse(response)
}

func loginCommand(args []string) error {
	flags := flag.NewFlagSet("login", flag.ExitOnError)
	username := flags.String("username", "", "NJU username")
	passwordCommand := flags.String("password-command", "", "command that prints the credential to stdout")
	keychainService := flags.String("keychain-service", "", "macOS Keychain generic-password service name")
	ask := flags.Bool("ask", false, "read credential interactively without echo")
	stdin := flags.Bool("password-stdin", false, "read credential from stdin")
	addQuick := flags.String("add-quick", "", "add current device as a quick-login device after login")
	_ = flags.Parse(args)
	if err := applyCredentialDefaults(username, keychainService, *passwordCommand, *ask, *stdin); err != nil {
		return err
	}
	secret, err := resolveSecret(*passwordCommand, *keychainService, *username, *ask, *stdin)
	if err != nil {
		return err
	}
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return err
	}
	data, err := client.postJSON("/api/portal/v1/login", map[string]string{
		"username": *username,
		"password": secret,
	})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	if *addQuick != "" {
		data, err = client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *addQuick})
		if err != nil {
			return err
		}
		printOperationResponse(data)
	}
	return nil
}

func setupCommand(args []string) error {
	if runtime.GOOS != "darwin" {
		return errors.New("setup requires macOS Keychain")
	}
	flags := flag.NewFlagSet("setup", flag.ExitOnError)
	username := flags.String("username", "", "NJU username")
	keychainService := flags.String("keychain-service", "", "macOS Keychain generic-password service name")
	_ = flags.Parse(args)
	existing, err := loadCredentialConfig()
	if err != nil {
		return err
	}
	if *username == "" && existing != nil {
		*username = existing.Username
	}
	if *keychainService == "" {
		if existing != nil && existing.KeychainService != "" {
			*keychainService = existing.KeychainService
		} else {
			*keychainService = defaultKeychainService
		}
	}
	if *username == "" {
		value, err := promptLine("NJU username: ")
		if err != nil {
			return err
		}
		*username = value
	}
	*username = strings.TrimSpace(*username)
	if *username == "" {
		return errors.New("empty username")
	}
	secret, err := promptSecret("NJU credential: ")
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("empty password")
	}
	if err := storeKeychainPassword(*keychainService, *username, secret); err != nil {
		return err
	}
	if err := saveCredentialConfig(credentialConfig{
		Username:        *username,
		KeychainService: *keychainService,
	}); err != nil {
		return err
	}
	path, err := configFilePath()
	if err != nil {
		return err
	}
	fmt.Println("state=configured")
	fmt.Printf("account=%s\n", maskAccount(*username))
	fmt.Printf("keychain_service=%s\n", *keychainService)
	fmt.Printf("config_path=%s\n", path)
	return nil
}

func logoutCommand() error {
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return err
	}
	data, err := client.postJSON("/api/portal/v1/logout", map[string]string{"domain": "default"})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	return nil
}

func infoCommand() error {
	if err := statusCommand(); err != nil {
		return err
	}
	fmt.Println()
	rows, err := onlineRowsLocal()
	if err != nil {
		return err
	}
	if err := printOnlineRows(rows); err != nil {
		return err
	}
	fmt.Println()
	response, err := getJSON(fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	if response.ReplyCode == 404 {
		fmt.Println("quick_login_devices=0")
		return nil
	}
	var quick listResults
	if err := json.Unmarshal(response.Results, &quick); err != nil {
		return err
	}
	return printQuickRows(quick.Rows)
}

func kickCommand(args []string) error {
	flags := flag.NewFlagSet("kick", flag.ExitOnError)
	id := flags.String("id", "", "online device id")
	index := flags.Int("index", 0, "1-based online device index from `njulogin info`")
	_ = flags.Parse(args)
	if *id != "" && *index != 0 {
		return errors.New("choose only one of --id or --index")
	}
	if *id == "" && *index == 0 {
		return errors.New("missing --id or --index")
	}
	if *index != 0 {
		rows, err := onlineRowsLocal()
		if err != nil {
			return err
		}
		resolved, err := onlineIDAtIndex(rows, *index)
		if err != nil {
			return err
		}
		*id = resolved
	}
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return err
	}
	data, err := client.postJSON("/api/selfservice/v1/online/kick/"+url.PathEscape(*id), map[string]string{})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	return nil
}

func quickCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: njulogin quick [list|add|delete]")
	}
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		response, err := getJSON(fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
		if err != nil {
			return err
		}
		if response.ReplyCode == 404 {
			fmt.Println("quick_login_devices=0")
			return nil
		}
		var results listResults
		if err := json.Unmarshal(response.Results, &results); err != nil {
			return err
		}
		return printQuickRows(results.Rows)
	case "add":
		flags := flag.NewFlagSet("quick add", flag.ExitOnError)
		name := flags.String("name", "", "quick-login device name")
		_ = flags.Parse(args[1:])
		if *name == "" {
			return errors.New("missing --name")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *name})
		if err != nil {
			return err
		}
		printOperationResponse(data)
		return nil
	case "delete":
		flags := flag.NewFlagSet("quick delete", flag.ExitOnError)
		id := flags.String("id", "", "quick-login device id")
		_ = flags.Parse(args[1:])
		if *id == "" {
			return errors.New("missing --id")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/unbind/id/"+url.PathEscape(*id), map[string]string{})
		if err != nil {
			return err
		}
		printOperationResponse(data)
		return nil
	default:
		return fmt.Errorf("unknown quick operation: %s", args[0])
	}
}

func qrCommand(args []string) error {
	flags := flag.NewFlagSet("qr", flag.ExitOnError)
	timeout := flags.Duration("timeout", 60*time.Second, "QR login timeout")
	_ = flags.Parse(args)
	client, err := newPortalClient("")
	if err != nil {
		return err
	}
	loginURL := casBaseURL + "/login?service=" + url.QueryEscape(casDestURL)
	req, err := http.NewRequest(http.MethodGet, loginURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	htmlData, err := do(req, client.http)
	if err != nil {
		return err
	}
	fields := parseLoginFields(string(htmlData))
	tokenURL := fmt.Sprintf("%s/qrCode/getToken?ts=%d", casBaseURL, time.Now().UnixMilli())
	req, err = http.NewRequest(http.MethodGet, tokenURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	tokenData, err := do(req, client.http)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(string(tokenData))
	if token == "" {
		return errors.New("empty QR token")
	}
	qrURL := fmt.Sprintf("%s/qrCode/qrCodeLogin.do?uuid=%s", casBaseURL, url.QueryEscape(token))
	qr, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		return err
	}
	fmt.Print(qr.ToSmallString(false))
	fmt.Println("scan_state=waiting")
	deadline := time.Now().Add(*timeout)
	for time.Now().Before(deadline) {
		statusURL := fmt.Sprintf("%s/qrCode/getStatus.htl?ts=%d&uuid=%s", casBaseURL, time.Now().UnixMilli(), url.QueryEscape(token))
		req, err = http.NewRequest(http.MethodGet, statusURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", userAgent)
		statusData, err := do(req, client.http)
		if err != nil {
			return err
		}
		switch strings.TrimSpace(string(statusData)) {
		case "1":
			return finishQRLogin(client.http, loginURL, fields, token)
		case "2":
			fmt.Println("scan_state=confirmed_on_phone_pending")
		case "3":
			return errors.New("QR code expired")
		}
		time.Sleep(time.Second)
	}
	return errors.New("QR login timed out")
}

func finishQRLogin(client *http.Client, loginURL string, fields loginFields, token string) error {
	form := url.Values{}
	form.Set("lt", fields.LT)
	form.Set("uuid", token)
	form.Set("cllt", "qrLogin")
	form.Set("dllt", fields.DLLT)
	form.Set("execution", fields.Execution)
	form.Set("_eventId", fields.EventID)
	form.Set("rmShown", fields.RMShown)
	req, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	if _, err := doWithoutRedirects(req, client); err != nil {
		return err
	}
	fmt.Println("state=qr_login_submitted")
	response, err := getPortalResponse(client, envDefault("NJU_PORTAL_BASE_URL", portalBaseURL), fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	return printStatusResponse(response)
}

func parseLoginFields(document string) loginFields {
	return loginFields{
		LT:        html.UnescapeString(inputValue(document, "lt")),
		DLLT:      html.UnescapeString(inputValue(document, "dllt")),
		Execution: html.UnescapeString(inputValue(document, "execution")),
		EventID:   html.UnescapeString(inputValue(document, "_eventId")),
		RMShown:   html.UnescapeString(inputValue(document, "rmShown")),
	}
}

func inputValue(document string, name string) string {
	pattern := regexp.MustCompile(`(?is)<input[^>]+name=["']` + regexp.QuoteMeta(name) + `["'][^>]*>`)
	valuePattern := regexp.MustCompile(`(?is)value=["']([^"']*)["']`)
	tag := pattern.FindString(document)
	if tag == "" {
		return ""
	}
	match := valuePattern.FindStringSubmatch(tag)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func printOperationResponse(data []byte) {
	response, err := parsePortalResponse(data)
	if err != nil {
		fmt.Println("operation_response=unparsed")
		return
	}
	fmt.Printf("reply_code=%d\n", response.ReplyCode)
	if response.ReplyMsg != "" {
		fmt.Printf("reply_msg=%s\n", response.ReplyMsg)
	}
}

func printStatusResponse(response *portalResponse) error {
	fmt.Printf("reply_code=%d\n", response.ReplyCode)
	if response.ReplyMsg != "" {
		fmt.Printf("reply_msg=%s\n", response.ReplyMsg)
	}
	rows, authenticated, err := portalSessionRows(response)
	if err != nil || !authenticated {
		fmt.Println("state=not_authenticated")
		return nil
	}
	fmt.Println("state=authenticated")
	fmt.Printf("sessions=%d\n", len(rows))
	for index, row := range rows {
		fmt.Printf(
			"session_%d=%s %s start=%s service=%s mac=%s ip=redacted\n",
			index+1,
			maskAccount(row.Username),
			maskText(row.Fullname),
			formatUnix(row.AcctStartTime),
			valueDefault(row.ServiceName, "unknown"),
			maskMAC(row.MAC),
		)
	}
	return nil
}

func portalSessionRows(response *portalResponse) ([]sessionRow, bool, error) {
	if response.ReplyCode != 0 {
		return nil, false, nil
	}
	var results getInfoResults
	if err := json.Unmarshal(response.Results, &results); err != nil {
		return nil, false, err
	}
	if len(results.Rows) == 0 {
		return nil, false, nil
	}
	return results.Rows, true, nil
}

func printQuickRows(rows []map[string]any) error {
	fmt.Printf("quick_login_devices=%d\n", len(rows))
	for _, row := range rows {
		fmt.Printf("quick device_id=%v mac=%s name=%v\n", row["id"], maskMAC(fmt.Sprint(row["mac"])), row["device"])
	}
	return nil
}

func printOnlineRows(rows []map[string]any) error {
	fmt.Printf("online_devices=%d\n", len(rows))
	for index, row := range rows {
		fmt.Printf(
			"online index=%d start=%s mac=%s ipv4=redacted ipv6=%s\n",
			index+1,
			formatAnyUnix(row["acctstarttime"]),
			maskMAC(fmt.Sprint(row["mac"])),
			ipv6Presence(row["user_ipv6"]),
		)
	}
	return nil
}

func resolveSecret(command, keychainService, username string, ask, stdin bool) (string, error) {
	switch {
	case command != "":
		output, err := exec.Command("bash", "-lc", command).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(output), "\r\n"), nil
	case stdin:
		reader := bufio.NewReader(os.Stdin)
		value, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(value, "\r\n"), nil
	case ask:
		fmt.Fprint(os.Stderr, "NJU credential: ")
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case keychainService != "":
		return readKeychainPassword(keychainService, username)
	default:
		return "", errors.New("choose one of --password-command, --keychain-service, --password-stdin, or --ask")
	}
}

func applyCredentialDefaults(username, keychainService *string, passwordCommand string, ask, stdin bool) error {
	config, err := loadCredentialConfig()
	if err != nil {
		return err
	}
	if config != nil {
		if *username == "" {
			*username = config.Username
		}
		if *keychainService == "" {
			*keychainService = config.KeychainService
		}
	}
	if *keychainService == "" && passwordCommand == "" && !ask && !stdin {
		*keychainService = defaultKeychainService
	}
	if strings.TrimSpace(*username) == "" {
		return errors.New("missing username: run `njulogin setup --username <username>` first or pass --username")
	}
	*username = strings.TrimSpace(*username)
	return nil
}

func configFilePath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("NJULOGIN_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "njulogin", "config.json"), nil
}

func loadCredentialConfig() (*credentialConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var config credentialConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	config.Username = strings.TrimSpace(config.Username)
	config.KeychainService = strings.TrimSpace(config.KeychainService)
	if config.Username == "" {
		return nil, fmt.Errorf("invalid config %s: missing username", path)
	}
	if config.KeychainService == "" {
		config.KeychainService = defaultKeychainService
	}
	return &config, nil
}

func saveCredentialConfig(config credentialConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	config.Username = strings.TrimSpace(config.Username)
	config.KeychainService = strings.TrimSpace(config.KeychainService)
	if config.Username == "" {
		return errors.New("cannot save config with empty username")
	}
	if config.KeychainService == "" {
		config.KeychainService = defaultKeychainService
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func readKeychainPassword(service, username string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("--keychain-service requires macOS")
	}
	output, err := exec.Command("security", "find-generic-password", "-s", service, "-a", username, "-w").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("security find-generic-password failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\r\n"), nil
}

func storeKeychainPassword(service, username, password string) error {
	cmd := keychainStoreCommand(service, username, password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("security add-generic-password failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func keychainStoreCommand(service, username, password string) *exec.Cmd {
	cmd := exec.Command(
		"security",
		"add-generic-password",
		"-U",
		"-a", username,
		"-s", service,
		"-l", service,
		"-w",
	)
	cmd.Stdin = strings.NewReader(password + "\n")
	return cmd
}

func promptLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func promptSecret(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("interactive password prompt requires a terminal")
	}
	fmt.Fprint(os.Stderr, prompt)
	data, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func promptMenuChoice(prompt string, options []menuOption, defaultKey string) (string, error) {
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, option.Key+"="+option.Label)
	}
	value, err := promptLine(fmt.Sprintf("%s [%s] (default %s): ", prompt, strings.Join(labels, ", "), defaultKey))
	if err != nil {
		return "", err
	}
	return resolveMenuChoice(value, defaultKey, options)
}

func resolveMenuChoice(input, defaultKey string, options []menuOption) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		value = defaultKey
	}
	for _, option := range options {
		if value == option.Key {
			return option.Key, nil
		}
	}
	return "", fmt.Errorf("unknown choice: %s", value)
}

func promptRequired(prompt string) (string, error) {
	value, err := promptLine(prompt)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("empty input")
	}
	return value, nil
}

func promptQuickLoginName(defaultValue string) (string, error) {
	if defaultValue == "" {
		defaultValue = "current-device"
	}
	value, err := promptLine(fmt.Sprintf("Quick-login device name (default %s): ", defaultValue))
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func interactiveCommand() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("interactive mode requires a terminal")
	}
	target, err := promptMenuChoice("Target", []menuOption{
		{Key: "1", Label: "local"},
		{Key: "2", Label: "ssh"},
	}, "1")
	if err != nil {
		return err
	}
	host := ""
	if target == "2" {
		host, err = promptRequired("SSH host: ")
		if err != nil {
			return err
		}
	}
	action, err := promptMenuChoice("Action", []menuOption{
		{Key: "1", Label: "password-login"},
		{Key: "2", Label: "qr-login"},
		{Key: "3", Label: "info"},
		{Key: "4", Label: "logout"},
		{Key: "5", Label: "kick-online"},
		{Key: "6", Label: "update-credential-and-login"},
		{Key: "7", Label: "password-login-and-bind-quick"},
	}, "1")
	if err != nil {
		return err
	}
	if action == "6" {
		if err := setupCommand(nil); err != nil {
			return err
		}
		action = "1"
	}
	if target == "1" {
		return runInteractiveLocal(action)
	}
	return runInteractiveRemote(host, action)
}

func runInteractiveLocal(action string) error {
	switch action {
	case "1":
		return loginCommand(nil)
	case "2":
		return qrCommand(nil)
	case "3":
		return infoCommand()
	case "4":
		return logoutCommand()
	case "5":
		if err := infoCommand(); err != nil {
			return err
		}
		index, err := promptRequired("Online device index to kick: ")
		if err != nil {
			return err
		}
		return kickCommand([]string{"--index", index})
	case "7":
		hostname, _ := os.Hostname()
		name, err := promptQuickLoginName(hostname)
		if err != nil {
			return err
		}
		return loginCommand([]string{"--add-quick", name})
	default:
		return fmt.Errorf("unsupported local action: %s", action)
	}
}

func runInteractiveRemote(host, action string) error {
	settings := defaultRemoteSettings()
	switch action {
	case "1":
		return remoteLoginCommand(host, settings, nil, false)
	case "2":
		return remoteQRCommand(host, settings)
	case "3":
		return remoteInfoCommand(host, settings)
	case "4":
		return remoteLogoutCommand(host, settings)
	case "5":
		if err := remoteInfoCommand(host, settings); err != nil {
			return err
		}
		index, err := promptRequired("Online device index to kick: ")
		if err != nil {
			return err
		}
		return remoteKickCommand(host, settings, []string{"--index", index})
	case "7":
		name, err := promptQuickLoginName(host)
		if err != nil {
			return err
		}
		return remoteLoginCommand(host, settings, []string{"--add-quick", name}, false)
	default:
		return fmt.Errorf("unsupported remote action: %s", action)
	}
}

func maskText(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return "unknown"
	}
	if len(runes) == 1 {
		return "*"
	}
	if len(runes) == 2 {
		return string(runes[:1]) + "*"
	}
	return string(runes[:1]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1:])
}

func maskAccount(value string) string {
	if len(value) <= 6 {
		return maskText(value)
	}
	return value[:3] + "***" + value[len(value)-3:]
}

func maskMAC(value string) string {
	parts := strings.Split(value, ":")
	if len(parts) != 6 {
		if value == "" {
			return "unknown"
		}
		return "redacted"
	}
	return strings.Join([]string{parts[0], parts[1], "xx", "xx", "xx", parts[5]}, ":")
}

func formatUnix(value int64) string {
	if value <= 0 {
		return "unknown"
	}
	return time.Unix(value, 0).Format("2006-01-02 15:04:05")
}

func valueDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func defaultRemoteSettings() remoteSettings {
	return remoteSettings{
		BaseURL:        envDefault("NJU_PORTAL_BASE_URL", portalBaseURL),
		Proxy:          os.Getenv("PNJU_CURL_PROXY"),
		Insecure:       os.Getenv("PNJU_INSECURE") == "1",
		ConnectTimeout: 5,
		MaxTime:        15,
	}
}

func remoteCommand(args []string) error {
	flags := flag.NewFlagSet("remote", flag.ExitOnError)
	settings := defaultRemoteSettings()
	flags.StringVar(&settings.BaseURL, "base-url", settings.BaseURL, "remote portal base URL")
	flags.StringVar(&settings.Proxy, "proxy", settings.Proxy, "remote curl proxy URL")
	flags.BoolVar(&settings.Insecure, "insecure", settings.Insecure, "remote curl -k")
	flags.IntVar(&settings.ConnectTimeout, "connect-timeout", settings.ConnectTimeout, "remote curl connect timeout")
	flags.IntVar(&settings.MaxTime, "max-time", settings.MaxTime, "remote curl max time")
	_ = flags.Parse(args)
	rest := flags.Args()
	if len(rest) < 2 {
		return errors.New("usage: njulogin remote [flags] <ssh-host> <operation> [args]")
	}
	host := rest[0]
	op := rest[1]
	opArgs := rest[2:]
	switch op {
	case "status":
		return remoteStatusCommand(host, settings)
	case "probe":
		return remoteProbeCommand(host, settings)
	case "login":
		return remoteLoginCommand(host, settings, opArgs, false)
	case "keychain-login":
		return remoteLoginCommand(host, settings, opArgs, true)
	case "logout":
		return remoteLogoutCommand(host, settings)
	case "info":
		return remoteInfoCommand(host, settings)
	case "quick":
		return remoteQuickCommand(host, settings, opArgs)
	case "add":
		return remoteQuickCommand(host, settings, append([]string{"add"}, opArgs...))
	case "delete":
		return remoteQuickCommand(host, settings, append([]string{"delete"}, opArgs...))
	case "kick":
		return remoteKickCommand(host, settings, opArgs)
	case "qr", "qr-login":
		return remoteQRCommand(host, settings)
	default:
		return fmt.Errorf("unknown remote operation: %s", op)
	}
}

func remoteStatusCommand(host string, settings remoteSettings) error {
	response, err := remoteGetJSON(host, settings, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	return printStatusResponse(response)
}

func remoteProbeCommand(host string, settings remoteSettings) error {
	fmt.Println("== status ==")
	if err := remoteStatusCommand(host, settings); err != nil {
		return err
	}
	return nil
}

func remoteLoginCommand(host string, settings remoteSettings, args []string, keychainDefault bool) error {
	flags := flag.NewFlagSet("remote login", flag.ExitOnError)
	username := flags.String("username", "", "NJU username")
	passwordCommand := flags.String("password-command", "", "command that prints the credential to stdout")
	keychainService := flags.String("keychain-service", "", "macOS Keychain generic-password service name")
	ask := flags.Bool("ask", false, "read credential interactively without echo")
	stdin := flags.Bool("password-stdin", false, "read credential from stdin")
	addQuick := flags.String("add-quick", "", "add current device as a quick-login device after login")
	_ = flags.Parse(args)
	if keychainDefault && *keychainService == "" {
		*keychainService = defaultKeychainService
	}
	if err := applyCredentialDefaults(username, keychainService, *passwordCommand, *ask, *stdin); err != nil {
		return err
	}
	secret, err := resolveSecret(*passwordCommand, *keychainService, *username, *ask, *stdin)
	if err != nil {
		return err
	}
	data, err := remotePostJSON(host, settings, "/api/portal/v1/login", map[string]string{
		"username": *username,
		"password": secret,
	})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	if *addQuick != "" {
		data, err = remotePostJSON(host, settings, "/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *addQuick})
		if err != nil {
			return err
		}
		printOperationResponse(data)
	}
	response, err := waitForRemoteAuthentication(host, settings, 10*time.Second)
	if err != nil {
		if response != nil {
			_ = printStatusResponse(response)
		}
		return err
	}
	return printStatusResponse(response)
}

func remoteLogoutCommand(host string, settings remoteSettings) error {
	data, err := remotePostJSON(host, settings, "/api/portal/v1/logout", map[string]string{"domain": "default"})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	return nil
}

func remoteInfoCommand(host string, settings remoteSettings) error {
	if err := remoteStatusCommand(host, settings); err != nil {
		return err
	}
	fmt.Println()
	response, err := remoteGetJSON(host, settings, fmt.Sprintf("/api/selfservice/v1/online?page=1&limit=10000&_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	var online listResults
	if err := json.Unmarshal(response.Results, &online); err != nil {
		return err
	}
	if err := printOnlineRows(online.Rows); err != nil {
		return err
	}
	fmt.Println()
	response, err = remoteGetJSON(host, settings, fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	if response.ReplyCode == 404 {
		fmt.Println("quick_login_devices=0")
		return nil
	}
	var quick listResults
	if err := json.Unmarshal(response.Results, &quick); err != nil {
		return err
	}
	return printQuickRows(quick.Rows)
}

func remoteQuickCommand(host string, settings remoteSettings, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: njulogin remote <host> quick [list|add|delete]")
	}
	switch args[0] {
	case "list":
		response, err := remoteGetJSON(host, settings, fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
		if err != nil {
			return err
		}
		if response.ReplyCode == 404 {
			fmt.Println("quick_login_devices=0")
			return nil
		}
		var quick listResults
		if err := json.Unmarshal(response.Results, &quick); err != nil {
			return err
		}
		return printQuickRows(quick.Rows)
	case "add":
		flags := flag.NewFlagSet("remote quick add", flag.ExitOnError)
		name := flags.String("name", "", "quick-login device name")
		_ = flags.Parse(args[1:])
		if *name == "" {
			return errors.New("missing --name")
		}
		data, err := remotePostJSON(host, settings, "/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *name})
		if err != nil {
			return err
		}
		printOperationResponse(data)
		return nil
	case "delete":
		flags := flag.NewFlagSet("remote quick delete", flag.ExitOnError)
		id := flags.String("id", "", "quick-login device id")
		_ = flags.Parse(args[1:])
		if *id == "" {
			return errors.New("missing --id")
		}
		data, err := remotePostJSON(host, settings, "/api/portal/v1/quicklogin/unbind/id/"+url.PathEscape(*id), map[string]string{})
		if err != nil {
			return err
		}
		printOperationResponse(data)
		return nil
	default:
		return fmt.Errorf("unknown remote quick operation: %s", args[0])
	}
}

func remoteKickCommand(host string, settings remoteSettings, args []string) error {
	flags := flag.NewFlagSet("remote kick", flag.ExitOnError)
	id := flags.String("id", "", "online device id")
	index := flags.Int("index", 0, "1-based online device index from `njulogin remote <host> info`")
	_ = flags.Parse(args)
	if *id != "" && *index != 0 {
		return errors.New("choose only one of --id or --index")
	}
	if *id == "" && *index == 0 {
		return errors.New("missing --id or --index")
	}
	if *index != 0 {
		rows, err := remoteOnlineRows(host, settings)
		if err != nil {
			return err
		}
		resolved, err := onlineIDAtIndex(rows, *index)
		if err != nil {
			return err
		}
		*id = resolved
	}
	data, err := remotePostJSON(host, settings, "/api/selfservice/v1/online/kick/"+url.PathEscape(*id), map[string]string{})
	if err != nil {
		return err
	}
	printOperationResponse(data)
	return nil
}

func remoteQRCommand(host string, settings remoteSettings) error {
	remoteCmd := shellJoin([]string{
		"env",
		"NJU_PORTAL_BASE_URL=" + settings.BaseURL,
		"NJU_CAS_BASE_URL=" + casBaseURL,
		"NJU_CAS_DEST_URL=" + casDestURL,
		"NJU_QR_TIMEOUT=" + strconv.Itoa(settings.MaxTime),
		"NJU_QR_LOGIN_TIMEOUT=60",
		"NJU_USER_AGENT=" + userAgent,
		"NJU_INSECURE=" + boolString(settings.Insecure),
		"sh",
		"-c",
		"if command -v python3 >/dev/null 2>&1; then exec python3 -; elif command -v python >/dev/null 2>&1; then exec python -; else echo 'missing remote python runtime' >&2; exit 127; fi",
	})
	cmd := exec.Command("ssh", "-T", host, remoteCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_, _ = io.Copy(os.Stderr, stderr)
	}()
	go func() {
		_, _ = io.WriteString(stdin, remoteQRBootstrap)
		_ = stdin.Close()
	}()
	reader := bufio.NewReader(stdout)
	header, err := reader.ReadString('\n')
	if err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("remote QR bootstrap did not return a QR URL: %w", err)
	}
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "QR_URL=") {
		_ = cmd.Wait()
		return fmt.Errorf("unexpected remote QR header: %s", header)
	}
	qrURL := strings.TrimPrefix(header, "QR_URL=")
	qr, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		_ = cmd.Wait()
		return err
	}
	fmt.Print(qr.ToSmallString(false))
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func waitForRemoteAuthentication(host string, settings remoteSettings, timeout time.Duration) (*portalResponse, error) {
	deadline := time.Now().Add(timeout)
	var last *portalResponse
	for {
		response, err := remoteGetJSON(host, settings, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
		if err == nil {
			last = response
			_, authenticated, parseErr := portalSessionRows(response)
			if parseErr == nil && authenticated {
				return response, nil
			}
		}
		if time.Now().After(deadline) {
			if last != nil {
				return last, errors.New("remote login accepted but authentication did not become active before timeout")
			}
			return nil, errors.New("remote login accepted but status polling failed before timeout")
		}
		time.Sleep(time.Second)
	}
}

func remoteOnlineRows(host string, settings remoteSettings) ([]map[string]any, error) {
	response, err := remoteGetJSON(host, settings, fmt.Sprintf("/api/selfservice/v1/online?page=1&limit=10000&_=%d", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}
	var online listResults
	if err := json.Unmarshal(response.Results, &online); err != nil {
		return nil, err
	}
	return online.Rows, nil
}

func onlineRowsLocal() ([]map[string]any, error) {
	response, err := getJSON(fmt.Sprintf("/api/selfservice/v1/online?page=1&limit=10000&_=%d", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}
	var online listResults
	if err := json.Unmarshal(response.Results, &online); err != nil {
		return nil, err
	}
	return online.Rows, nil
}

func onlineIDAtIndex(rows []map[string]any, index int) (string, error) {
	if index <= 0 {
		return "", errors.New("--index must be >= 1")
	}
	if index > len(rows) {
		return "", fmt.Errorf("--index %d out of range; online_devices=%d", index, len(rows))
	}
	id := strings.TrimSpace(fmt.Sprint(rows[index-1]["id"]))
	if id == "" || id == "<nil>" {
		return "", fmt.Errorf("online device %d has no usable id", index)
	}
	return id, nil
}

func remoteGetJSON(host string, settings remoteSettings, path string) (*portalResponse, error) {
	data, err := sshCapture(host, buildRemoteCurlCommand(settings, http.MethodGet, path, false), nil)
	if err != nil {
		return nil, err
	}
	return parsePortalResponse(data)
}

func remotePostJSON(host string, settings remoteSettings, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return sshCapture(host, buildRemoteCurlCommand(settings, http.MethodPost, path, true), data)
}

func buildRemoteCurlCommand(settings remoteSettings, method, path string, dataFromStdin bool) string {
	args := []string{
		"curl",
		"-sS",
		"--connect-timeout", strconv.Itoa(settings.ConnectTimeout),
		"--max-time", strconv.Itoa(settings.MaxTime),
	}
	if settings.Insecure {
		args = append(args, "-k")
	}
	if settings.Proxy != "" {
		args = append(args, "-x", settings.Proxy)
	} else {
		args = append(args, "--noproxy", "*")
	}
	if method != http.MethodGet {
		args = append(args, "-X", method)
	}
	if dataFromStdin {
		args = append(args, "-H", "Content-Type: application/json", "--data", "@-")
	}
	args = append(args, strings.TrimRight(settings.BaseURL, "/")+path)
	return shellJoin(args)
}

func sshCapture(host, remoteCmd string, stdin []byte) ([]byte, error) {
	cmd := exec.Command("ssh", host, remoteCmd)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ssh %s failed: %w: %s", host, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func sshStream(host, remoteCmd string) error {
	cmd := exec.Command("ssh", "-tt", host, remoteCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  njulogin setup [--username USER] [--keychain-service SERVICE]
  njulogin status
  njulogin info
  njulogin login [--username USER] [--keychain-service SERVICE|--password-command CMD|--password-stdin|--ask] [--add-quick NAME]
  njulogin qr
  njulogin logout
  njulogin kick --id ID|--index N
  njulogin quick list
  njulogin quick add --name NAME
  njulogin quick delete --id ID
  njulogin remote [flags] <ssh-host> <operation>
  njulogin version`)
}

func main() {
	if len(os.Args) < 2 {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			usage()
			os.Exit(2)
		}
		if err := interactiveCommand(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	var err error
	switch os.Args[1] {
	case "setup":
		err = setupCommand(os.Args[2:])
	case "status":
		err = statusCommand()
	case "info":
		err = infoCommand()
	case "login":
		err = loginCommand(os.Args[2:])
	case "qr":
		err = qrCommand(os.Args[2:])
	case "logout":
		err = logoutCommand()
	case "kick":
		err = kickCommand(os.Args[2:])
	case "quick":
		err = quickCommand(os.Args[2:])
	case "remote":
		err = remoteCommand(os.Args[2:])
	case "version", "--version":
		fmt.Printf("njulogin %s\n", version)
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func formatAnyUnix(value any) string {
	switch cast := value.(type) {
	case float64:
		return formatUnix(int64(cast))
	case int64:
		return formatUnix(cast)
	case int:
		return formatUnix(int64(cast))
	default:
		return "unknown"
	}
}

func ipv6Presence(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "none"
	}
	return "present"
}
