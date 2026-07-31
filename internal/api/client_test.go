package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func zeroBackoff(int, string) time.Duration { return 0 }

func newTestClient(baseURL string, opts ...Option) (*Client, error) {
	return New(baseURL, "perm:test", append([]Option{WithBackoff(zeroBackoff)}, opts...)...)
}

func TestNewInvalidBaseURL(t *testing.T) {
	for _, u := range []string{"", "://bad", "localhost:8080", "ftp://x"} {
		if _, err := New(u, ""); err == nil {
			t.Errorf("New(%q): expected error", u)
		}
	}
}

func TestNewTrailingSlashTrimmed(t *testing.T) {
	c, err := New("http://example.com/api/", "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if c.baseURL != "http://example.com/api" {
		t.Errorf("baseURL = %q, want without trailing slash", c.baseURL)
	}
}

func TestGetQueryParams(t *testing.T) {
	var mu sync.Mutex
	var gotPath, gotRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		gotRaw = r.URL.RawQuery
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	q := url.Values{"fields": {"id,login,fullName"}, "$top": {"30"}, "$skip": {"5"}}
	if err := c.Get(context.Background(), "/users/me", q, &map[string]any{}); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/users/me" {
		t.Errorf("path = %q, want /users/me", gotPath)
	}
	if gotRaw != "$skip=5&$top=30&fields=id,login,fullName" {
		t.Errorf("raw query = %q, want $skip=5&$top=30&fields=id,login,fullName", gotRaw)
	}
}

func TestGetQueryParamEscaping(t *testing.T) {
	var gotRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	q := url.Values{"fields": {"id,customFields(value($type,name))"}}
	if err := c.Get(context.Background(), "/issues", q, &map[string]any{}); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if gotRaw != "fields=id,customFields(value($type,name))" {
		t.Errorf("raw query = %q, want literal $ and parentheses", gotRaw)
	}
}

func TestAuthorizationHeader(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	_ = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if gotAuth != "Bearer perm:test" {
		t.Errorf("Authorization = %q, want Bearer perm:test", gotAuth)
	}
}

func TestNoAuthorizationHeaderWithoutToken(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := New(ts.URL, "")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}

func TestUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	_ = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if !strings.HasPrefix(gotUA, "yt/") {
		t.Errorf("User-Agent = %q, want prefix yt/", gotUA)
	}
}

func TestGetDecodes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1-1","login":"alex"}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	var out struct {
		ID    string `json:"id"`
		Login string `json:"login"`
	}
	if err := c.Get(context.Background(), "/users/me", nil, &out); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if out.ID != "1-1" || out.Login != "alex" {
		t.Errorf("decoded = %+v, want {1-1 alex}", out)
	}
}

func TestPostJSON(t *testing.T) {
	var (
		gotMethod, gotCT, gotBody string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		_, _ = w.Write([]byte(`{"id":"2-1"}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	body := []byte(`{"summary":"Fix login flow"}`)
	if err := c.Post(context.Background(), "/issues", nil, body, &map[string]any{}); err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if gotBody != string(body) {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
}

func TestDelete(t *testing.T) {
	var gotMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if err := c.Delete(context.Background(), "/issues/2-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
}

func TestPathMustStartWithSlash(t *testing.T) {
	c, err := newTestClient("http://example.com")
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if err := c.Get(context.Background(), "users/me", nil, &map[string]any{}); err == nil {
		t.Error("Get() with path without leading slash: expected error")
	}
}

func TestErrorParsing(t *testing.T) {
	cases := []struct {
		name    string
		code    int
		body    string
		wantErr string
	}{
		{"auth", 401, `{"error":"Authentication required","error_description":"No token"}`, `not logged in or token is invalid, run "yt auth login"`},
		{"notfound", 404, `{"error":"Issue PRJ-999 not found"}`, "request failed: 404 Issue PRJ-999 not found"},
		{"http403", 403, `{"error":"You don't have permission to edit"}`, "request failed: 403 You don't have permission to edit"},
		{"http500", 500, `{"error":"boom","error_description":"something went wrong"}`, "request failed: 500 boom: something went wrong"},
		{"emptybody", 400, ``, "request failed: 400 Bad Request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			c, err := newTestClient(ts.URL)
			if err != nil {
				t.Fatalf("newTestClient error: %v", err)
			}
			err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
			var ae *Error
			if !errors.As(err, &ae) {
				t.Fatalf("error = %v, want *api.Error", err)
			}
			if ae.Error() != tc.wantErr {
				t.Errorf("Error() = %q, want %q", ae.Error(), tc.wantErr)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want ErrorType
	}{
		{"401", 401, ErrorAuth},
		{"404", 404, ErrorNotFound},
		{"403", 403, ErrorHTTP},
		{"500", 500, ErrorHTTP},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"error":"x"}`))
			}))
			defer ts.Close()
			c, err := newTestClient(ts.URL)
			if err != nil {
				t.Fatalf("newTestClient error: %v", err)
			}
			err = c.Get(context.Background(), "/x", nil, &map[string]any{})
			var ae *Error
			if !errors.As(err, &ae) {
				t.Fatalf("error = %v, want *api.Error", err)
			}
			if ae.Type != tc.want {
				t.Errorf("Type = %v, want %v", ae.Type, tc.want)
			}
		})
	}
}

func TestRetryGETOn5xx(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if err := c.Get(context.Background(), "/x", nil, &map[string]any{}); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3", calls.Load())
	}
}

func TestRetryGETOn429(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	if err := c.Get(context.Background(), "/x", nil, &map[string]any{}); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestNoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	err = c.Get(context.Background(), "/x", nil, &map[string]any{})
	if err == nil {
		t.Fatal("Get() expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

func TestNoRetryOnPost(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	err = c.Post(context.Background(), "/issues", nil, []byte(`{}`), &map[string]any{})
	if err == nil {
		t.Fatal("Post() expected error")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

type fakeRoundTripper struct {
	fn func(r *http.Request) (*http.Response, error)
	n  atomic.Int32
}

func (f *fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	f.n.Add(1)
	return f.fn(r)
}

func TestRetryOnNetworkError(t *testing.T) {
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}}
	c, err := New("http://example.com/api", "perm:test",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithBackoff(zeroBackoff),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if rt.n.Load() != 3 {
		t.Errorf("attempts = %d, want 3", rt.n.Load())
	}
	var ae *Error
	if !errors.As(err, &ae) || ae.Type != ErrorNetwork {
		t.Fatalf("error = %v, want *api.Error with Type ErrorNetwork", err)
	}
	if !strings.Contains(ae.Error(), "cannot reach http://example.com/api") {
		t.Errorf("Error() = %q, want contains cannot reach", ae.Error())
	}
}

func TestNoRetryNetworkOnPost(t *testing.T) {
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}}
	c, err := New("http://example.com/api", "perm:test",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithBackoff(zeroBackoff),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	err = c.Post(context.Background(), "/issues", nil, []byte(`{}`), &map[string]any{})
	if rt.n.Load() != 1 {
		t.Errorf("attempts = %d, want 1", rt.n.Load())
	}
	if err == nil {
		t.Fatal("Post() expected error")
	}
}

func TestRetryAfterHeaderUsed(t *testing.T) {
	var gotRA string
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		h := http.Header{}
		h.Set("Retry-After", "0")
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     h,
			Body:       http.NoBody,
			Request:    r,
		}, nil
	}}
	c, err := New("http://example.com/api", "perm:test",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithBackoff(func(attempt int, retryAfter string) time.Duration {
			gotRA = retryAfter
			return 0
		}),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if err == nil {
		t.Fatal("Get() expected error")
	}
	if gotRA != "0" {
		t.Errorf("backoff retryAfter = %q, want 0", gotRA)
	}
	if rt.n.Load() != 3 {
		t.Errorf("attempts = %d, want 3", rt.n.Load())
	}
}

func TestRetryGivesUp(t *testing.T) {
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		h := http.Header{}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     h,
			Body:       http.NoBody,
			Request:    r,
		}, nil
	}}
	c, err := New("http://example.com/api", "perm:test",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithBackoff(zeroBackoff),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if err == nil {
		t.Fatal("Get() expected error")
	}
	var ae *Error
	if !errors.As(err, &ae) || ae.Code != http.StatusInternalServerError {
		t.Errorf("error = %v, want 500", err)
	}
	if rt.n.Load() != 3 {
		t.Errorf("attempts = %d, want 3", rt.n.Load())
	}
}

func TestContextCanceled(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = c.Get(ctx, "/users/me", nil, &map[string]any{})
	var ae *Error
	if !errors.As(err, &ae) || ae.Type != ErrorCanceled {
		t.Fatalf("error = %v, want ErrorCanceled", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, want true")
	}
	if calls.Load() != 0 {
		t.Errorf("server calls = %d, want 0", calls.Load())
	}
}

func TestCancelDuringBackoff(t *testing.T) {
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}}
	c, err := New("http://example.com/api", "perm:test",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithBackoff(func(int, string) time.Duration { return time.Hour }),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = c.Get(ctx, "/users/me", nil, &map[string]any{})
	var ae *Error
	if !errors.As(err, &ae) || ae.Type != ErrorCanceled {
		t.Fatalf("error = %v, want ErrorCanceled", err)
	}
}

func TestDecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(``))
	}))
	defer ts.Close()

	c, err := newTestClient(ts.URL)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	err = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	var ae *Error
	if !errors.As(err, &ae) || ae.Type != ErrorHTTP {
		t.Fatalf("error = %v, want ErrorHTTP", err)
	}
	if ae.Code != http.StatusOK {
		t.Errorf("Code = %d, want 200", ae.Code)
	}
}

func TestLoggerDoesNotLeakToken(t *testing.T) {
	var logged strings.Builder
	rt := &fakeRoundTripper{fn: func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       http.NoBody,
			Request:    r,
		}, nil
	}}
	c, err := New("http://example.com/api", "perm:super-secret-token",
		WithHTTPClient(&http.Client{Transport: rt}),
		WithLogger(func(format string, args ...any) {
			logged.WriteString(format + "\n")
		}),
	)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = c.Get(context.Background(), "/users/me", nil, &map[string]any{})
	if strings.Contains(logged.String(), "super-secret-token") {
		t.Errorf("logger leaked token: %q", logged.String())
	}
}

func TestEscapePath(t *testing.T) {
	cases := map[string]string{
		"PRJ-42": "PRJ-42",
		"a b":    "a%20b",
		"a/b":    "a%2Fb",
	}
	for in, want := range cases {
		if got := EscapePath(in); got != want {
			t.Errorf("EscapePath(%q) = %q, want %q", in, got, want)
		}
	}
}
