package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amolofeev/prompt-and-pray/internal/version"
)

const (
	DefaultMaxRetries = 3
	defaultTimeout    = 30 * time.Second
	connectTimeout    = 10 * time.Second
)

type ErrorType int

const (
	ErrorNetwork ErrorType = iota
	ErrorAuth
	ErrorNotFound
	ErrorHTTP
	ErrorCanceled
)

type Error struct {
	Type    ErrorType
	Code    int
	Title   string
	Detail  string
	BaseURL string
	Err     error
}

func (e *Error) Error() string {
	switch e.Type {
	case ErrorNetwork:
		if e.Err != nil {
			return fmt.Sprintf("cannot reach %s: %v", e.BaseURL, e.Err)
		}
		return fmt.Sprintf("cannot reach %s", e.BaseURL)
	case ErrorAuth:
		return `not logged in or token is invalid, run "yt auth login"`
	case ErrorCanceled:
		if e.Err != nil {
			return e.Err.Error()
		}
		return "canceled"
	default:
		msg := e.Title
		if msg == "" {
			msg = http.StatusText(e.Code)
		}
		if e.Detail != "" && e.Detail != msg {
			msg += ": " + e.Detail
		}
		return fmt.Sprintf("request failed: %d %s", e.Code, msg)
	}
}

func (e *Error) Unwrap() error { return e.Err }

type Client struct {
	baseURL    string
	token      string
	http       *http.Client
	timeout    time.Duration
	maxRetries int
	backoff    func(attempt int, retryAfter string) time.Duration
	logf       func(format string, args ...any)
}

type Option func(*Client)

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

func WithBackoff(fn func(attempt int, retryAfter string) time.Duration) Option {
	return func(c *Client) { c.backoff = fn }
}

func WithLogger(logf func(format string, args ...any)) Option {
	return func(c *Client) { c.logf = logf }
}

func New(baseURL, token string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid base URL %q", baseURL)
	}
	c := &Client{baseURL: baseURL, token: token}
	for _, opt := range opts {
		opt(c)
	}
	if c.timeout == 0 {
		c.timeout = defaultTimeout
	}
	if c.http == nil {
		c.http = &http.Client{Transport: defaultTransport(), Timeout: c.timeout}
	}
	if c.maxRetries == 0 {
		c.maxRetries = DefaultMaxRetries
	}
	if c.backoff == nil {
		c.backoff = defaultBackoff
	}
	if c.logf == nil {
		c.logf = func(string, ...any) {}
	}
	return c, nil
}

func defaultTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext
	return t
}

func EscapePath(s string) string {
	return url.PathEscape(s)
}

func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, query url.Values, body []byte, out any) error {
	return c.do(ctx, http.MethodPost, path, query, body, out)
}

func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
	u, err := c.urlFor(path, query)
	if err != nil {
		return err
	}
	maxAttempts := 1
	if method == http.MethodGet {
		maxAttempts = c.maxRetries
	}
	var (
		lastErr    error
		retryAfter string
	)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := c.wait(ctx, attempt-1, retryAfter); err != nil {
				return err
			}
		}
		resp, err := c.roundTrip(ctx, method, u, body)
		if err != nil {
			lastErr = err
			retryAfter = ""
			if c.retryable(method, err) {
				continue
			}
			return err
		}
		if resp.StatusCode/100 == 2 {
			if out != nil {
				if err := decodeJSON(resp, out); err != nil {
					return &Error{Type: ErrorHTTP, Code: resp.StatusCode, BaseURL: c.baseURL, Title: "decode response", Err: err}
				}
			} else {
				drainAndClose(resp)
			}
			return nil
		}
		apierr := parseError(resp)
		lastErr = apierr
		if c.retryable(method, apierr) {
			retryAfter = resp.Header.Get("Retry-After")
			continue
		}
		return apierr
	}
	return lastErr
}

func (c *Client) urlFor(path string, query url.Values) (string, error) {
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("path must start with %q: %q", "/", path)
	}
	u := c.baseURL + path
	if q := encodeQuery(query); q != "" {
		u += "?" + q
	}
	return u, nil
}

func (c *Client) roundTrip(ctx context.Context, method, u string, body []byte) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "yt/"+version.Version)
	start := time.Now()
	resp, err := c.http.Do(req)
	dur := time.Since(start).Round(time.Millisecond)
	if err != nil {
		if ctx.Err() != nil {
			c.logf("DBG %s %s err=%s dur=%s", method, u, context.Canceled, dur)
			return nil, &Error{Type: ErrorCanceled, Err: ctx.Err()}
		}
		c.logf("DBG %s %s err=%s dur=%s", method, u, err, dur)
		return nil, &Error{Type: ErrorNetwork, BaseURL: c.baseURL, Err: err}
	}
	c.logf("DBG %s %s status=%d dur=%s", method, u, resp.StatusCode, dur)
	return resp, nil
}

func (c *Client) retryable(method string, err error) bool {
	if method != http.MethodGet {
		return false
	}
	var ae *Error
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Type {
	case ErrorNetwork:
		return true
	case ErrorHTTP:
		return ae.Code == http.StatusTooManyRequests || ae.Code >= http.StatusInternalServerError
	}
	return false
}

func (c *Client) wait(ctx context.Context, attempt int, retryAfter string) error {
	d := c.backoff(attempt, retryAfter)
	if d <= 0 {
		select {
		case <-ctx.Done():
			return &Error{Type: ErrorCanceled, Err: ctx.Err()}
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return &Error{Type: ErrorCanceled, Err: ctx.Err()}
	case <-t.C:
		return nil
	}
}

func defaultBackoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	delay := (500 * time.Millisecond) << attempt
	return delay + time.Duration(rand.Intn(101)-50)*time.Millisecond
}

func encodeQuery(v url.Values) string {
	if len(v) == 0 {
		return ""
	}
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	first := true
	for _, k := range keys {
		for _, val := range v[k] {
			if !first {
				b.WriteByte('&')
			}
			first = false
			b.WriteString(escapeQueryComponent(k))
			b.WriteByte('=')
			b.WriteString(escapeQueryComponent(val))
		}
	}
	return b.String()
}

func escapeQueryComponent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if isQuerySafe(ch) {
			b.WriteByte(ch)
		} else {
			b.WriteString(url.QueryEscape(string(ch)))
		}
	}
	return b.String()
}

func isQuerySafe(ch byte) bool {
	switch {
	case ch >= 'a' && ch <= 'z':
		return true
	case ch >= 'A' && ch <= 'Z':
		return true
	case ch >= '0' && ch <= '9':
		return true
	}
	switch ch {
	case '-', '_', '.', '~', '$', ',', '(', ')', '!', '\'', '*', ';':
		return true
	}
	return false
}

func decodeJSON(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()
	return json.NewDecoder(resp.Body).Decode(out)
}

func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}

type serverError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func parseError(resp *http.Response) *Error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
	var se serverError
	_ = json.Unmarshal(data, &se)
	e := &Error{
		Type:   ErrorHTTP,
		Code:   resp.StatusCode,
		Title:  se.Error,
		Detail: se.ErrorDescription,
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		e.Type = ErrorAuth
	case http.StatusNotFound:
		e.Type = ErrorNotFound
	}
	return e
}
