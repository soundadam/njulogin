package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
	"unicode"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/term"
)

const (
	portalBaseURL        = "https://p.nju.edu.cn"
	casBaseURL           = "https://authserver.nju.edu.cn/authserver"
	casDestURL           = "https://p.nju.edu.cn:443/api/cas/getinfo/&renew=true"
	maxResponseBodyBytes = 1 << 20
	remoteQRLoginTimeout = 60 * time.Second
	maxConnectTimeout    = 300
	maxRequestTimeout    = 3600
)

var userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
var version = "dev"
var postconditionTimeout = 10 * time.Second
var postconditionPollInterval = 250 * time.Millisecond
var passwordCommandTimeout = 15 * time.Second
var localCASBaseURL = casBaseURL
var localCASDestURL = casDestURL
var credentialPrompt = promptSecret

type portalClient struct {
	baseURL string
	http    *http.Client
}

type portalResponse struct {
	ReplyCode int             `json:"reply_code"`
	Results   json.RawMessage `json:"results"`
}

type getInfoResults struct {
	Rows *[]sessionRow `json:"rows"`
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
	Rows *[]map[string]any `json:"rows"`
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
	Insecure       bool
	ConnectTimeout int
	MaxTime        int
}

type credentialConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type menuOption struct {
	Key   string
	Label string
}

type usageError struct {
	message string
}

func (err usageError) Error() string {
	return err.message
}

func usagef(format string, args ...any) error {
	return usageError{message: fmt.Sprintf(format, args...)}
}

func exitCodeForError(err error) int {
	var syntax usageError
	if errors.As(err, &syntax) {
		return 2
	}
	return 1
}

func requireNoPositionals(flags *flag.FlagSet) error {
	if flags.NArg() != 0 {
		return usagef("unexpected positional arguments for %s: %s", flags.Name(), strings.Join(flags.Args(), " "))
	}
	return nil
}

func requireNoArgs(command string, args []string) error {
	if len(args) != 0 {
		return usagef("%s does not accept arguments", command)
	}
	return nil
}

func newPortalClient(baseURL string) (*portalClient, error) {
	if err := validatePortalBaseURL(baseURL); err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	}
	return &portalClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout:   15 * time.Second,
			Jar:       jar,
			Transport: transport,
		},
	}, nil
}

func validatePortalBaseURL(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return usagef("invalid portal base URL: %v", err)
	}
	if parsed.User != nil {
		return usagef("portal base URL must not contain embedded credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return usagef("portal base URL must not contain a query or fragment")
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
	return usagef("portal base URL must use https, except for an HTTP loopback test server")
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
		return nil, errors.New("HTTP request failed")
	}
	defer resp.Body.Close()
	data, err := readResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
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
		return nil, errors.New("HTTP request failed")
	}
	defer resp.Body.Close()
	data, err := readResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return nil, fmt.Errorf("HTTP request returned status %d", resp.StatusCode)
	}
	return data, nil
}

func readResponseBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBodyBytes {
		return nil, fmt.Errorf("HTTP response exceeds %d bytes", maxResponseBodyBytes)
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
	return &portalResponse{
		ReplyCode: *wire.ReplyCode,
		Results:   wire.Results,
	}, nil
}

func getPortalResponse(client *http.Client, baseURL, path string) (*portalResponse, error) {
	if err := validatePortalBaseURL(baseURL); err != nil {
		return nil, err
	}
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

func quickRowsWithClient(client *http.Client, baseURL string) ([]map[string]any, error) {
	response, err := getPortalResponse(client, baseURL, fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}
	return quickRowsFromResponse(response)
}

func onlineRowsWithClient(client *http.Client, baseURL string) ([]map[string]any, error) {
	response, err := getPortalResponse(client, baseURL, fmt.Sprintf("/api/selfservice/v1/online?page=1&limit=10000&_=%d", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}
	return onlineRowsFromResponse(response)
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
	ask := flags.Bool("ask", false, "read credential interactively without echo")
	stdin := flags.Bool("password-stdin", false, "read credential from stdin")
	addQuick := flags.String("add-quick", "", "add current device as a quick-login device after login")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
	}
	secret, err := resolveLoginCredential(username, *passwordCommand, *ask, *stdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return usagef("empty password")
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
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	response, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return getPortalResponse(client.http, client.baseURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	}, true, postconditionTimeout)
	if err != nil {
		return err
	}
	if *addQuick != "" {
		data, err = client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *addQuick})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return quickNamePresent(rows, *addQuick) },
			postconditionTimeout,
			"quick-login binding was not observed before timeout",
		); err != nil {
			return err
		}
	}
	if err := printStatusResponse(response); err != nil {
		return err
	}
	if *addQuick != "" {
		fmt.Println("quick_add_result=success")
	}
	return nil
}

func setupCommand(args []string) error {
	flags := flag.NewFlagSet("setup", flag.ExitOnError)
	username := flags.String("username", "", "NJU username")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
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
		return usagef("empty username")
	}
	secret, err := credentialPrompt("NJU credential: ")
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return usagef("empty password")
	}
	if err := saveCredentialConfig(credentialConfig{Username: *username, Password: secret}); err != nil {
		return err
	}
	path, err := configFilePath()
	if err != nil {
		return err
	}
	fmt.Println("state=configured")
	fmt.Printf("account=%s\n", maskAccount(*username))
	fmt.Println("credential_store=local_file")
	fmt.Printf("config_path=%s\n", outputField(path, "unknown"))
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
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	response, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return getPortalResponse(client.http, client.baseURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	}, false, postconditionTimeout)
	if err != nil {
		return err
	}
	return printStatusResponse(response)
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
	quick, err := quickRowsFromResponse(response)
	if err != nil {
		return err
	}
	return printQuickRows(quick)
}

func kickCommand(args []string) error {
	flags := flag.NewFlagSet("kick", flag.ExitOnError)
	id := flags.String("id", "", "online device id")
	index := flags.Int("index", 0, "1-based online device index from `njulogin info`")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
	}
	if *id != "" && *index != 0 {
		return usagef("choose only one of --id or --index")
	}
	if *id == "" && *index == 0 {
		return usagef("missing --id or --index")
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
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	if err := waitForRows(
		func() ([]map[string]any, error) { return onlineRowsWithClient(client.http, client.baseURL) },
		func(rows []map[string]any) bool { return rowIDAbsent(rows, *id) },
		postconditionTimeout,
		"online-session target remained present before timeout",
	); err != nil {
		return err
	}
	fmt.Println("result=success")
	return nil
}

func quickCommand(args []string) error {
	if len(args) == 0 {
		return usagef("usage: njulogin quick [list|add|delete]")
	}
	client, err := newPortalClient(envDefault("NJU_PORTAL_BASE_URL", portalBaseURL))
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usagef("quick list does not accept arguments")
		}
		response, err := getJSON(fmt.Sprintf("/api/portal/v1/quicklogin/domain/default?_=%d", time.Now().UnixMilli()))
		if err != nil {
			return err
		}
		rows, err := quickRowsFromResponse(response)
		if err != nil {
			return err
		}
		return printQuickRows(rows)
	case "add":
		flags := flag.NewFlagSet("quick add", flag.ExitOnError)
		name := flags.String("name", "", "quick-login device name")
		_ = flags.Parse(args[1:])
		if err := requireNoPositionals(flags); err != nil {
			return err
		}
		if *name == "" {
			return usagef("missing --name")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *name})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return quickNamePresent(rows, *name) },
			postconditionTimeout,
			"quick-login binding was not observed before timeout",
		); err != nil {
			return err
		}
		fmt.Println("result=success")
		return nil
	case "delete":
		flags := flag.NewFlagSet("quick delete", flag.ExitOnError)
		id := flags.String("id", "", "quick-login device id")
		_ = flags.Parse(args[1:])
		if err := requireNoPositionals(flags); err != nil {
			return err
		}
		if *id == "" {
			return usagef("missing --id")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/unbind/id/"+url.PathEscape(*id), map[string]string{})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return rowIDAbsent(rows, *id) },
			postconditionTimeout,
			"quick-login binding remained present before timeout",
		); err != nil {
			return err
		}
		fmt.Println("result=success")
		return nil
	default:
		return usagef("unknown quick operation: %s", args[0])
	}
}

func qrCommand(args []string) error {
	client, err := newPortalClient("")
	if err != nil {
		return err
	}
	return qrCommandWithClient(
		args,
		client.http,
		envDefault("NJU_PORTAL_BASE_URL", portalBaseURL),
		localCASBaseURL,
		localCASDestURL,
	)
}

func qrCommandWithClient(args []string, client *http.Client, portalURL, casURL, destinationURL string) error {
	flags := flag.NewFlagSet("qr", flag.ExitOnError)
	timeout := flags.Duration("timeout", 60*time.Second, "QR login timeout")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
	}
	if *timeout <= 0 {
		return usagef("--timeout must be positive")
	}
	if err := validatePortalBaseURL(portalURL); err != nil {
		return err
	}
	if err := validatePortalBaseURL(casURL); err != nil {
		return usagef("invalid CAS base URL: %v", err)
	}
	loginURL := strings.TrimRight(casURL, "/") + "/login?service=" + url.QueryEscape(destinationURL)
	req, err := http.NewRequest(http.MethodGet, loginURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	htmlData, err := do(req, client)
	if err != nil {
		return fmt.Errorf("load CAS login page: %w", err)
	}
	fields := parseLoginFields(string(htmlData))
	if fields.LT == "" || fields.Execution == "" || fields.EventID == "" {
		return errors.New("CAS login page is missing required form fields")
	}
	tokenURL := fmt.Sprintf("%s/qrCode/getToken?ts=%d", strings.TrimRight(casURL, "/"), time.Now().UnixMilli())
	req, err = http.NewRequest(http.MethodGet, tokenURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	tokenData, err := do(req, client)
	if err != nil {
		return fmt.Errorf("request QR token: %w", err)
	}
	token := strings.TrimSpace(string(tokenData))
	if token == "" {
		return errors.New("empty QR token")
	}
	qrURL := fmt.Sprintf("%s/qrCode/qrCodeLogin.do?uuid=%s", strings.TrimRight(casURL, "/"), url.QueryEscape(token))
	qr, err := qrcode.New(qrURL, qrcode.Medium)
	if err != nil {
		return errors.New("generate QR code from portal token")
	}
	fmt.Print(qr.ToSmallString(false))
	fmt.Println("scan_state=waiting")
	deadline := time.Now().Add(*timeout)
	confirmedReported := false
	for time.Now().Before(deadline) {
		statusURL := fmt.Sprintf("%s/qrCode/getStatus.htl?ts=%d&uuid=%s", strings.TrimRight(casURL, "/"), time.Now().UnixMilli(), url.QueryEscape(token))
		req, err = http.NewRequest(http.MethodGet, statusURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", userAgent)
		statusData, err := do(req, client)
		if err != nil {
			return fmt.Errorf("poll QR status: %w", err)
		}
		switch strings.TrimSpace(string(statusData)) {
		case "1":
			return finishQRLogin(client, portalURL, loginURL, fields, token)
		case "2":
			if !confirmedReported {
				fmt.Println("scan_state=confirmed_on_phone_pending")
				confirmedReported = true
			}
		case "3":
			return errors.New("QR code expired")
		}
		time.Sleep(time.Second)
	}
	return errors.New("QR login timed out")
}

func finishQRLogin(client *http.Client, portalURL, loginURL string, fields loginFields, token string) error {
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
		return fmt.Errorf("submit QR login: %w", err)
	}
	fmt.Println("scan_state=login_submitted")
	response, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return getPortalResponse(client, portalURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	}, true, postconditionTimeout)
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

func requireOperationSuccess(data []byte) error {
	response, err := parsePortalResponse(data)
	if err != nil {
		return fmt.Errorf("parse portal operation response: %w", err)
	}
	if response.ReplyCode != 0 {
		return fmt.Errorf("portal operation rejected with reply_code=%d", response.ReplyCode)
	}
	return nil
}

func printStatusResponse(response *portalResponse) error {
	rows, authenticated, err := portalSessionRows(response)
	if err != nil {
		return err
	}
	if !authenticated {
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
			outputField(row.ServiceName, "unknown"),
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
	if results.Rows == nil {
		return nil, false, errors.New("portal status response is missing results.rows")
	}
	if len(*results.Rows) == 0 {
		return nil, false, nil
	}
	return *results.Rows, true, nil
}

func quickRowsFromResponse(response *portalResponse) ([]map[string]any, error) {
	if response.ReplyCode == 404 {
		return []map[string]any{}, nil
	}
	if response.ReplyCode != 0 {
		return nil, fmt.Errorf("portal quick-login query failed with reply_code=%d", response.ReplyCode)
	}
	var results listResults
	if err := json.Unmarshal(response.Results, &results); err != nil {
		return nil, err
	}
	if results.Rows == nil {
		return nil, errors.New("portal quick-login response is missing results.rows")
	}
	return *results.Rows, nil
}

func onlineRowsFromResponse(response *portalResponse) ([]map[string]any, error) {
	if response.ReplyCode != 0 {
		return nil, fmt.Errorf("portal online-session query failed with reply_code=%d", response.ReplyCode)
	}
	var results listResults
	if err := json.Unmarshal(response.Results, &results); err != nil {
		return nil, err
	}
	if results.Rows == nil {
		return nil, errors.New("portal online-session response is missing results.rows")
	}
	return *results.Rows, nil
}

func waitForAuthenticationState(fetch func() (*portalResponse, error), wantAuthenticated bool, timeout time.Duration) (*portalResponse, error) {
	deadline := time.Now().Add(timeout)
	for {
		response, err := fetch()
		if err != nil {
			return nil, err
		}
		_, authenticated, err := portalSessionRows(response)
		if err != nil {
			return response, err
		}
		if authenticated == wantAuthenticated {
			return response, nil
		}
		if !time.Now().Before(deadline) {
			if wantAuthenticated {
				return response, errors.New("operation accepted but authenticated state was not observed before timeout")
			}
			return response, errors.New("operation accepted but unauthenticated state was not observed before timeout")
		}
		time.Sleep(postconditionPollInterval)
	}
}

func waitForRows(fetch func() ([]map[string]any, error), predicate func([]map[string]any) bool, timeout time.Duration, failure string) error {
	deadline := time.Now().Add(timeout)
	for {
		rows, err := fetch()
		if err != nil {
			return err
		}
		if predicate(rows) {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errors.New(failure)
		}
		time.Sleep(postconditionPollInterval)
	}
}

func rowIDAbsent(rows []map[string]any, id string) bool {
	for _, row := range rows {
		if strings.TrimSpace(fmt.Sprint(row["id"])) == id {
			return false
		}
	}
	return true
}

func quickNamePresent(rows []map[string]any, name string) bool {
	for _, row := range rows {
		if strings.TrimSpace(fmt.Sprint(row["device"])) == name {
			return true
		}
	}
	return false
}

func printQuickRows(rows []map[string]any) error {
	fmt.Printf("quick_login_devices=%d\n", len(rows))
	for index, row := range rows {
		fmt.Printf(
			"quick_%d=device_id=%v mac=%s name=%v\n",
			index+1,
			outputField(fmt.Sprint(row["id"]), "unknown"),
			maskMAC(fmt.Sprint(row["mac"])),
			outputField(fmt.Sprint(row["device"]), "unknown"),
		)
	}
	return nil
}

func printOnlineRows(rows []map[string]any) error {
	fmt.Printf("online_devices=%d\n", len(rows))
	for index, row := range rows {
		fmt.Printf(
			"online_%d=start=%s mac=%s ipv4=redacted ipv6=%s\n",
			index+1,
			formatAnyUnix(row["acctstarttime"]),
			maskMAC(fmt.Sprint(row["mac"])),
			ipv6Presence(row["user_ipv6"]),
		)
	}
	return nil
}

func resolveLoginCredential(username *string, command string, ask, stdin bool) (string, error) {
	explicitSources := 0
	if command != "" {
		explicitSources++
	}
	if ask {
		explicitSources++
	}
	if stdin {
		explicitSources++
	}
	if explicitSources > 1 {
		return "", usagef("credential sources are mutually exclusive: choose one of --password-command, --password-stdin, or --ask")
	}
	var config *credentialConfig
	if *username == "" || explicitSources == 0 {
		var err error
		config, err = loadCredentialConfig()
		if err != nil {
			return "", err
		}
	}
	if *username == "" && config != nil {
		*username = config.Username
	}
	*username = strings.TrimSpace(*username)
	if *username == "" {
		return "", usagef("missing username: run `njulogin setup --username <username>` first or pass --username")
	}
	if explicitSources != 0 {
		return resolveExplicitSecret(command, ask, stdin)
	}
	if config == nil {
		return "", usagef("no saved credential: run `njulogin setup` or choose --password-command, --password-stdin, or --ask")
	}
	if config.Username != *username {
		return "", usagef("saved credential belongs to a different username; run `njulogin setup` or choose an explicit credential source")
	}
	if strings.TrimSpace(config.Password) == "" {
		return "", errors.New("saved credential is empty")
	}
	return config.Password, nil
}

func resolveExplicitSecret(command string, ask, stdin bool) (string, error) {
	switch {
	case command != "":
		ctx, cancel := context.WithTimeout(context.Background(), passwordCommandTimeout)
		defer cancel()
		output, err := newCommandContext(ctx, "bash", "-lc", command).Output()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("password command timed out")
		}
		if err != nil {
			return "", fmt.Errorf("password command failed: %w", err)
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
	default:
		return "", usagef("choose one of --password-command, --password-stdin, or --ask")
	}
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
	if err := validateConfigPermissions(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config credentialConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: trailing JSON data", path)
	}
	config.Username = strings.TrimSpace(config.Username)
	if config.Username == "" {
		return nil, fmt.Errorf("invalid config %s: missing username", path)
	}
	if strings.TrimSpace(config.Password) == "" {
		return nil, fmt.Errorf("invalid config %s: missing password", path)
	}
	return &config, nil
}

func validateConfigPermissions(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("config %s must be a regular file, not a symlink or special file", path)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		return fmt.Errorf("config %s has mode %04o; expected 0600", path, mode)
	}
	dir := filepath.Dir(path)
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return fmt.Errorf("config parent %s must be a directory, not a symlink or special file", dir)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		return fmt.Errorf("config directory %s has mode %04o; expected 0700", dir, mode)
	}
	return nil
}

func saveCredentialConfig(config credentialConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	config.Username = strings.TrimSpace(config.Username)
	if config.Username == "" {
		return errors.New("cannot save config with empty username")
	}
	if strings.TrimSpace(config.Password) == "" {
		return errors.New("cannot save config with empty password")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return fmt.Errorf("config parent %s must be a directory, not a symlink or special file", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("config %s must be a regular file, not a symlink or special file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
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
		return remoteLoginCommand(host, settings, nil)
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
		return remoteLoginCommand(host, settings, []string{"--add-quick", name})
	default:
		return fmt.Errorf("unsupported remote action: %s", action)
	}
}

func maskText(value string) string {
	runes := []rune(normalizeField(value))
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
	value = normalizeField(value)
	runes := []rune(value)
	if len(runes) <= 6 {
		return maskText(value)
	}
	return string(runes[:3]) + "***" + string(runes[len(runes)-3:])
}

func maskMAC(value string) string {
	value = normalizeField(value)
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

func normalizeField(value string) string {
	cleaned := strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return ' '
		}
		return char
	}, value)
	return strings.Join(strings.Fields(cleaned), " ")
}

func outputField(value, fallback string) string {
	value = normalizeField(value)
	if value == "" || value == "<nil>" {
		return fallback
	}
	return value
}

func effectiveVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return resolveVersion(version, nil)
	}
	return resolveVersion(version, info)
}

func resolveVersion(linked string, info *debug.BuildInfo) string {
	linked = strings.TrimSpace(linked)
	if linked != "" && linked != "dev" {
		return linked
	}
	if info != nil {
		moduleVersion := strings.TrimSpace(info.Main.Version)
		if moduleVersion != "" && moduleVersion != "(devel)" {
			return moduleVersion
		}
	}
	if linked == "" {
		return "dev"
	}
	return linked
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
		Insecure:       false,
		ConnectTimeout: 5,
		MaxTime:        15,
	}
}

func validateRemoteSettings(settings remoteSettings) error {
	if err := validatePortalBaseURL(settings.BaseURL); err != nil {
		return err
	}
	if settings.ConnectTimeout <= 0 {
		return usagef("--connect-timeout must be positive")
	}
	if settings.ConnectTimeout > maxConnectTimeout {
		return usagef("--connect-timeout must be at most %d seconds", maxConnectTimeout)
	}
	if settings.MaxTime <= 0 {
		return usagef("--max-time must be positive")
	}
	if settings.MaxTime > maxRequestTimeout {
		return usagef("--max-time must be at most %d seconds", maxRequestTimeout)
	}
	return nil
}

func validateSSHHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return usagef("missing SSH host")
	}
	if strings.IndexFunc(host, func(value rune) bool {
		return value <= ' ' || value == 0x7f
	}) >= 0 {
		return usagef("SSH host must not contain whitespace or control characters")
	}
	if strings.HasPrefix(host, "-") {
		return usagef("SSH host must not begin with '-'")
	}
	return nil
}

func remoteCommand(args []string) error {
	flags := flag.NewFlagSet("remote", flag.ExitOnError)
	settings := defaultRemoteSettings()
	flags.StringVar(&settings.BaseURL, "base-url", settings.BaseURL, "portal base URL reached through SSH")
	flags.BoolVar(&settings.Insecure, "insecure", settings.Insecure, "disable local TLS verification for tunneled requests")
	flags.IntVar(&settings.ConnectTimeout, "connect-timeout", settings.ConnectTimeout, "SSH connect timeout")
	flags.IntVar(&settings.MaxTime, "max-time", settings.MaxTime, "per-request timeout")
	_ = flags.Parse(args)
	rest := flags.Args()
	if len(rest) < 2 {
		return usagef("usage: njulogin remote [flags] <ssh-host> <operation> [args]")
	}
	host := rest[0]
	if err := validateSSHHost(host); err != nil {
		return err
	}
	if err := validateRemoteSettings(settings); err != nil {
		return err
	}
	if settings.Insecure {
		fmt.Fprintln(os.Stderr, "warning: TLS verification is disabled for locally processed tunneled requests")
	}
	op := rest[1]
	opArgs := rest[2:]
	switch op {
	case "status":
		if err := requireNoArgs("remote status", opArgs); err != nil {
			return err
		}
		return remoteStatusCommand(host, settings)
	case "login":
		return remoteLoginCommand(host, settings, opArgs)
	case "logout":
		if err := requireNoArgs("remote logout", opArgs); err != nil {
			return err
		}
		return remoteLogoutCommand(host, settings)
	case "info":
		if err := requireNoArgs("remote info", opArgs); err != nil {
			return err
		}
		return remoteInfoCommand(host, settings)
	case "quick":
		return remoteQuickCommand(host, settings, opArgs)
	case "kick":
		return remoteKickCommand(host, settings, opArgs)
	case "qr":
		if err := requireNoArgs("remote qr", opArgs); err != nil {
			return err
		}
		return remoteQRCommand(host, settings)
	default:
		return usagef("unknown remote operation: %s", op)
	}
}

func newRemotePortalClient(host string, settings remoteSettings) (*portalClient, error) {
	httpClient, err := newSSHHTTPClient(host, settings)
	if err != nil {
		return nil, err
	}
	return &portalClient{baseURL: strings.TrimRight(settings.BaseURL, "/"), http: httpClient}, nil
}

func remoteStatusWithClient(client *portalClient) error {
	response, err := getPortalResponse(client.http, client.baseURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	if err != nil {
		return err
	}
	return printStatusResponse(response)
}

func remoteStatusCommand(host string, settings remoteSettings) error {
	client, err := newRemotePortalClient(host, settings)
	if err != nil {
		return err
	}
	return remoteStatusWithClient(client)
}

func remoteLoginCommand(host string, settings remoteSettings, args []string) error {
	flags := flag.NewFlagSet("remote login", flag.ExitOnError)
	username := flags.String("username", "", "NJU username")
	passwordCommand := flags.String("password-command", "", "command that prints the credential to stdout")
	ask := flags.Bool("ask", false, "read credential interactively without echo")
	stdin := flags.Bool("password-stdin", false, "read credential from stdin")
	addQuick := flags.String("add-quick", "", "add current device as a quick-login device after login")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
	}
	secret, err := resolveLoginCredential(username, *passwordCommand, *ask, *stdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(secret) == "" {
		return usagef("empty password")
	}
	client, err := newRemotePortalClient(host, settings)
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
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	response, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return getPortalResponse(client.http, client.baseURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	}, true, postconditionTimeout)
	if err != nil {
		return err
	}
	if *addQuick != "" {
		data, err = client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *addQuick})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return quickNamePresent(rows, *addQuick) },
			postconditionTimeout,
			"remote quick-login binding was not observed before timeout",
		); err != nil {
			return err
		}
	}
	if err := printStatusResponse(response); err != nil {
		return err
	}
	if *addQuick != "" {
		fmt.Println("quick_add_result=success")
	}
	return nil
}

func remoteLogoutCommand(host string, settings remoteSettings) error {
	client, err := newRemotePortalClient(host, settings)
	if err != nil {
		return err
	}
	data, err := client.postJSON("/api/portal/v1/logout", map[string]string{"domain": "default"})
	if err != nil {
		return err
	}
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	response, err := waitForAuthenticationState(func() (*portalResponse, error) {
		return getPortalResponse(client.http, client.baseURL, fmt.Sprintf("/api/portal/v1/getinfo?_=%d", time.Now().UnixMilli()))
	}, false, postconditionTimeout)
	if err != nil {
		return err
	}
	return printStatusResponse(response)
}

func remoteInfoCommand(host string, settings remoteSettings) error {
	client, err := newRemotePortalClient(host, settings)
	if err != nil {
		return err
	}
	if err := remoteStatusWithClient(client); err != nil {
		return err
	}
	fmt.Println()
	rows, err := onlineRowsWithClient(client.http, client.baseURL)
	if err != nil {
		return err
	}
	if err := printOnlineRows(rows); err != nil {
		return err
	}
	fmt.Println()
	quick, err := quickRowsWithClient(client.http, client.baseURL)
	if err != nil {
		return err
	}
	return printQuickRows(quick)
}

func remoteQuickCommand(host string, settings remoteSettings, args []string) error {
	if len(args) == 0 {
		return usagef("usage: njulogin remote <host> quick [list|add|delete]")
	}
	client, err := newRemotePortalClient(host, settings)
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usagef("remote quick list does not accept arguments")
		}
		rows, err := quickRowsWithClient(client.http, client.baseURL)
		if err != nil {
			return err
		}
		return printQuickRows(rows)
	case "add":
		flags := flag.NewFlagSet("remote quick add", flag.ExitOnError)
		name := flags.String("name", "", "quick-login device name")
		_ = flags.Parse(args[1:])
		if err := requireNoPositionals(flags); err != nil {
			return err
		}
		if *name == "" {
			return usagef("missing --name")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/bind/default", map[string]string{"device": *name})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return quickNamePresent(rows, *name) },
			postconditionTimeout,
			"remote quick-login binding was not observed before timeout",
		); err != nil {
			return err
		}
		fmt.Println("result=success")
		return nil
	case "delete":
		flags := flag.NewFlagSet("remote quick delete", flag.ExitOnError)
		id := flags.String("id", "", "quick-login device id")
		_ = flags.Parse(args[1:])
		if err := requireNoPositionals(flags); err != nil {
			return err
		}
		if *id == "" {
			return usagef("missing --id")
		}
		data, err := client.postJSON("/api/portal/v1/quicklogin/unbind/id/"+url.PathEscape(*id), map[string]string{})
		if err != nil {
			return err
		}
		if err := requireOperationSuccess(data); err != nil {
			return err
		}
		if err := waitForRows(
			func() ([]map[string]any, error) { return quickRowsWithClient(client.http, client.baseURL) },
			func(rows []map[string]any) bool { return rowIDAbsent(rows, *id) },
			postconditionTimeout,
			"remote quick-login binding remained present before timeout",
		); err != nil {
			return err
		}
		fmt.Println("result=success")
		return nil
	default:
		return usagef("unknown remote quick operation: %s", args[0])
	}
}

func remoteKickCommand(host string, settings remoteSettings, args []string) error {
	flags := flag.NewFlagSet("remote kick", flag.ExitOnError)
	id := flags.String("id", "", "online device id")
	index := flags.Int("index", 0, "1-based online device index from `njulogin remote <host> info`")
	_ = flags.Parse(args)
	if err := requireNoPositionals(flags); err != nil {
		return err
	}
	if *id != "" && *index != 0 {
		return usagef("choose only one of --id or --index")
	}
	if *id == "" && *index == 0 {
		return usagef("missing --id or --index")
	}
	client, err := newRemotePortalClient(host, settings)
	if err != nil {
		return err
	}
	if *index != 0 {
		rows, err := onlineRowsWithClient(client.http, client.baseURL)
		if err != nil {
			return err
		}
		resolved, err := onlineIDAtIndex(rows, *index)
		if err != nil {
			return err
		}
		*id = resolved
	}
	data, err := client.postJSON("/api/selfservice/v1/online/kick/"+url.PathEscape(*id), map[string]string{})
	if err != nil {
		return err
	}
	if err := requireOperationSuccess(data); err != nil {
		return err
	}
	if err := waitForRows(
		func() ([]map[string]any, error) { return onlineRowsWithClient(client.http, client.baseURL) },
		func(rows []map[string]any) bool { return rowIDAbsent(rows, *id) },
		postconditionTimeout,
		"remote online-session target remained present before timeout",
	); err != nil {
		return err
	}
	fmt.Println("result=success")
	return nil
}

func remoteQRCommand(host string, settings remoteSettings) error {
	client, err := newSSHHTTPClient(host, settings)
	if err != nil {
		return err
	}
	return qrCommandWithClient(nil, client, settings.BaseURL, localCASBaseURL, localCASDestURL)
}

func onlineRowsLocal() ([]map[string]any, error) {
	response, err := getJSON(fmt.Sprintf("/api/selfservice/v1/online?page=1&limit=10000&_=%d", time.Now().UnixMilli()))
	if err != nil {
		return nil, err
	}
	return onlineRowsFromResponse(response)
}

func onlineIDAtIndex(rows []map[string]any, index int) (string, error) {
	if index <= 0 {
		return "", usagef("--index must be >= 1")
	}
	if index > len(rows) {
		return "", usagef("--index %d out of range; online_devices=%d", index, len(rows))
	}
	id := strings.TrimSpace(fmt.Sprint(rows[index-1]["id"]))
	if id == "" || id == "<nil>" {
		return "", usagef("online device %d has no usable id", index)
	}
	return id, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  njulogin setup [--username USER]
  njulogin status
  njulogin info
  njulogin login [--username USER] [--password-command CMD|--password-stdin|--ask] [--add-quick NAME]
  njulogin qr
  njulogin logout
  njulogin kick --id ID|--index N
  njulogin quick list
  njulogin quick add --name NAME
  njulogin quick delete --id ID
  njulogin remote [flags] <ssh-host> <operation>
  njulogin version`)
}

func run(args []string) error {
	if len(args) == 0 {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			usage()
			return usagef("interactive mode requires a terminal")
		}
		return interactiveCommand()
	}
	command := args[0]
	rest := args[1:]
	switch command {
	case "setup":
		return setupCommand(rest)
	case "status":
		if err := requireNoArgs("status", rest); err != nil {
			return err
		}
		return statusCommand()
	case "info":
		if err := requireNoArgs("info", rest); err != nil {
			return err
		}
		return infoCommand()
	case "login":
		return loginCommand(rest)
	case "qr":
		return qrCommand(rest)
	case "logout":
		if err := requireNoArgs("logout", rest); err != nil {
			return err
		}
		return logoutCommand()
	case "kick":
		return kickCommand(rest)
	case "quick":
		return quickCommand(rest)
	case "remote":
		return remoteCommand(rest)
	case "version", "--version":
		if err := requireNoArgs("version", rest); err != nil {
			return err
		}
		fmt.Printf("njulogin %s\n", effectiveVersion())
		return nil
	case "-h", "--help", "help":
		if err := requireNoArgs("help", rest); err != nil {
			return err
		}
		usage()
		return nil
	default:
		return usagef("unknown command: %s", command)
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeForError(err))
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
