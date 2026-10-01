// Package ai adds explanations to measured sorting results. Model output never
// controls the sorter, checker, filesystem, or shell.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const RequestTimeout = 10 * time.Second
const MaxResponseBytes = 1 << 20

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Config struct{ BaseURL, Model, APIKey string }

func FromEnvironment() Config {
	return Config{os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_MODEL"), os.Getenv("LLM_API_KEY")}
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config) *Client {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if strings.TrimSpace(config.Model) == "" {
		config.Model = "local-model"
	}
	return &Client{config: config, http: &http.Client{
		Timeout: RequestTimeout,
		// A redirect must not forward a credential to another endpoint.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}
func (c *Client) Configured() bool { return c.config.BaseURL != "" }

// Complete returns only a validated assistant response. Errors deliberately
// omit URLs, headers, response bodies and transport details that could contain
// credentials. The mode layer supplies a deterministic fallback on any error.
func (c *Client) Complete(ctx context.Context, messages []Message) (string, error) {
	base, err := url.Parse(c.config.BaseURL)
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return "", errors.New("LLM_BASE_URL must be an HTTP(S) base URL without credentials, query or fragment")
	}
	body, err := json.Marshal(struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}{c.config.Model, messages})
	if err != nil {
		return "", errors.New("could not encode model request")
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("could not build model request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if c.config.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.New("model request timed out or was cancelled")
		}
		return "", errors.New("model backend is unreachable or the connection failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("model backend returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes+1))
	if err != nil {
		return "", errors.New("could not read model response")
	}
	if len(data) > MaxResponseBytes {
		return "", errors.New("model response exceeded 1 MiB")
	}
	if !utf8.Valid(data) {
		return "", errors.New("model response is not UTF-8")
	}
	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil || len(decoded.Choices) == 0 {
		return "", errors.New("model returned malformed or empty JSON")
	}
	message := decoded.Choices[0].Message
	if message.Role != "" && message.Role != "assistant" {
		return "", errors.New("model response has no assistant message")
	}
	answer := strings.TrimSpace(terminalText(message.Content))
	if answer == "" {
		return "", errors.New("model returned an empty assistant message")
	}
	return answer, nil
}

// Keep terminal control characters from remote output out of the display.
func terminalText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, text)
}
