package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ernat-soltanbekov/swap-sort/internal/sorter"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (fn roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestTwoConfigurableBackends(t *testing.T) {
	for _, test := range []struct{ path, model, key string }{{"/v1", "first-model", ""}, {"/another/prefix", "second-model", "test-secret"}} {
		t.Run(test.model, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != test.path+"/chat/completions" {
					t.Error("wrong endpoint", r.Method, r.URL.Path)
				}
				wantAuth := ""
				if test.key != "" {
					wantAuth = "Bearer " + test.key
				}
				if r.Header.Get("Authorization") != wantAuth {
					t.Error("wrong authorization")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing JSON content type")
				}
				var body struct {
					Model    string    `json:"model"`
					Messages []Message `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Model != test.model || len(body.Messages) != 2 || body.Messages[0].Role != "system" || !strings.Contains(body.Messages[1].Content, "Operations") {
					t.Error("wrong payload", body)
				}
				io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Strategy: model response\nEfficiency: measured."}}]}`)
			}))
			defer server.Close()
			coach := New(Config{server.URL + test.path + "/", test.model, test.key})
			var out bytes.Buffer
			if err := coach.Run(context.Background(), "explain", []int64{3, 2, 1}, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "Backend: configured model") || !strings.Contains(out.String(), "Strategy: model response") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestProtocolFailuresFallback(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"rate-limit", 429, "private-server-details"}, {"server-error", 500, "secret"}, {"redirect", 302, ""},
		{"invalid-json", 200, "<html>error</html>"}, {"trailing-json", 200, `{"choices":[]} {}`},
		{"no-choices", 200, `{"choices":[]}`}, {"null", 200, "null"},
		{"no-message", 200, `{"choices":[{}]}`}, {"blank-message", 200, `{"choices":[{"message":{"content":"  "}}]}`},
		{"wrong-role", 200, `{"choices":[{"message":{"role":"user","content":"not an assistant"}}]}`},
		{"oversized", 200, strings.Repeat("x", MaxResponseBytes+1)}, {"bad-utf8", 200, string([]byte{0xff, 0xfe})},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			c := New(Config{BaseURL: "http://example.invalid/v1"})
			c.Client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return reply(test.status, test.body), nil })
			var out bytes.Buffer
			if err := c.Run(context.Background(), "explain", []int64{2, 1}, strings.NewReader(""), &out); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Backend unavailable:", "Backend: mock (offline)", "Strategy:", "Step by step:", "Efficiency:", "Code improvements:"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %s: %s", want, out.String())
				}
			}
			if strings.Contains(out.String(), "private-server-details") || strings.Contains(out.String(), "secret") {
				t.Fatal("response error body leaked")
			}
		})
	}
}

func TestNetworkFailureTimeoutAndNoSecretLeak(t *testing.T) {
	c := NewClient(Config{BaseURL: "http://example.invalid", APIKey: "private-token"})
	if c.http.Timeout != 10*time.Second {
		t.Fatal("required timeout is missing")
	}
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, errors.New("private-token")
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Complete(ctx, []Message{{"user", "test"}})
	if err == nil || !strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "private-token") || time.Since(start) > time.Second {
		t.Fatal(err)
	}
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("private-token") })
	_, err = c.Complete(context.Background(), nil)
	if err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("transport details leaked", err)
	}
}

func TestBadURLsAndRedirects(t *testing.T) {
	for _, base := range []string{"file:///etc/passwd", "localhost:1234", "https://user:secret@example.invalid", "http://example.invalid?key=secret", "http://example.invalid#secret", "%"} {
		c := NewClient(Config{BaseURL: base})
		if _, err := c.Complete(context.Background(), nil); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(base, err)
		}
	}
	var reached atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Store(true) }))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	c := NewClient(Config{BaseURL: origin.URL, APIKey: "private-token"})
	_, err := c.Complete(context.Background(), nil)
	if err == nil || reached.Load() {
		t.Fatal("redirect followed")
	}
}

func TestEnvironmentAndOfflineModes(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_API_KEY", "")
	c := New(FromEnvironment())
	if c.Client.Configured() || c.Client.config.Model != "local-model" {
		t.Fatal("wrong default")
	}
	c.Client.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("offline mode used network"); return nil, nil })
	for _, mode := range []string{"explain", "debug", "compare", "chat"} {
		var first, second bytes.Buffer
		input := "What do pa and pb do?\nWhat about tests?\nexit\n"
		for _, out := range []*bytes.Buffer{&first, &second} {
			if err := c.Run(context.Background(), mode, []int64{5, 4, 3, 2, 1}, strings.NewReader(input), out); err != nil {
				t.Fatal(err)
			}
		}
		if first.String() != second.String() {
			t.Fatal("mock is not deterministic")
		}
		if mode == "debug" && !strings.Contains(first.String(), "Result: OK") {
			t.Fatal(first.String())
		}
		if mode == "compare" && (!strings.Contains(first.String(), "Recommendation:") || !strings.Contains(first.String(), "estimate")) {
			t.Fatal(first.String())
		}
		if mode == "chat" && (!strings.Contains(first.String(), "1 earlier question(s) retained") || !strings.Contains(first.String(), "Code improvements:")) {
			t.Fatal(first.String())
		}
	}
	var out bytes.Buffer
	c.Run(context.Background(), "explain", []int64{1, 2, 3}, strings.NewReader(""), &out)
	if !strings.Contains(out.String(), "already sorted") || !strings.Contains(out.String(), "0 operations is optimal") {
		t.Fatal(out.String())
	}
}

func TestDebugFailureSuppliesFullTraceAndFix(t *testing.T) {
	for _, ops := range [][]string{{"pb"}, {"ra"}, {"bad"}} {
		c := New(Config{BaseURL: "http://example.invalid"})
		c.Sort = func([]int64) sorter.Result {
			return sorter.Result{Operations: ops, Strategy: "deliberately faulty test planner"}
		}
		called := false
		c.Client.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
			called = true
			data, _ := io.ReadAll(r.Body)
			for _, want := range []string{"Complete execution trace", "Initial:", "Input:", "Operations"} {
				if !strings.Contains(string(data), want) {
					t.Error("missing debug evidence", want)
				}
			}
			return reply(503, ""), nil
		})
		var out bytes.Buffer
		if err := c.Run(context.Background(), "debug", []int64{3, 2, 1}, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if !called || !strings.Contains(out.String(), "Result: KO") || !strings.Contains(out.String(), "Suggested fix:") {
			t.Fatal(out.String())
		}
	}
}

func TestChatPreservesMessagesAndExit(t *testing.T) {
	c := New(Config{BaseURL: "http://example.invalid"})
	calls := 0
	c.Client.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		var body struct {
			Messages []Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if calls == 1 && len(body.Messages) != 3 {
			t.Error("first context", body)
		}
		if calls == 2 && (len(body.Messages) != 5 || body.Messages[2].Content != "first question" || body.Messages[3].Role != "assistant" || body.Messages[3].Content != "first answer" || body.Messages[4].Content != "second question") {
			t.Error("lost conversation context", body)
		}
		return reply(200, `{"choices":[{"message":{"content":"first answer"}}]}`), nil
	})
	var out bytes.Buffer
	if err := c.Run(context.Background(), "chat", []int64{2, 1}, strings.NewReader("first question\nsecond question\nexit\nignored\n"), &out); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(out.String(), "Session ended") {
		t.Fatal(calls, out.String())
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestCoachLimitsAndIOFailures(t *testing.T) {
	c := New(Config{})
	for _, values := range [][]int64{nil, make([]int64, MaxCoachValues+1)} {
		if err := c.Run(context.Background(), "explain", values, strings.NewReader(""), io.Discard); err == nil {
			t.Fatal("input limit missing")
		}
	}
	if err := c.Run(context.Background(), "unknown", []int64{1}, strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if err := c.Run(context.Background(), "explain", []int64{2, 1}, strings.NewReader(""), brokenWriter{}); err == nil {
		t.Fatal("write failure ignored")
	}
	if err := c.Run(context.Background(), "chat", []int64{2, 1}, strings.NewReader(strings.Repeat("x", MaxChatMessageBytes+1)), io.Discard); err == nil {
		t.Fatal("oversized chat accepted")
	}
	var out bytes.Buffer
	if err := c.Run(context.Background(), "chat", []int64{2, 1}, strings.NewReader(strings.Repeat("tests\n", MaxChatTurns+1)), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Session limit reached") {
		t.Fatal("unbounded history")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Run(ctx, "chat", []int64{2, 1}, strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestTerminalControlsRemoved(t *testing.T) {
	c := NewClient(Config{BaseURL: "http://example.invalid"})
	c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
		return reply(200, `{"choices":[{"message":{"content":"hello\u001b[31m\u0007\r\nworld"}}]}`), nil
	})
	text, err := c.Complete(context.Background(), nil)
	if err != nil || strings.ContainsAny(text, "\x1b\a\r") {
		t.Fatal(text, err)
	}
}

func FuzzResponse(f *testing.F) {
	f.Add(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`)
	f.Add("null")
	f.Add("<html>")
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > MaxResponseBytes+1 {
			return
		}
		c := NewClient(Config{BaseURL: "http://example.invalid"})
		c.http.Transport = roundTrip(func(*http.Request) (*http.Response, error) { return reply(200, body), nil })
		text, err := c.Complete(context.Background(), nil)
		if err == nil && (strings.TrimSpace(text) == "" || strings.ContainsRune(text, '\x1b')) {
			t.Fatal("invalid response accepted")
		}
	})
}
