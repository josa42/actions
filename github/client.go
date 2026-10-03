// Package github is a minimal client for the GitHub REST API and the context
// of the running workflow.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxAttempts = 3
	// Rate limits resetting later than this fail instead of waiting.
	maxWait = time.Minute
)

// Client calls the GitHub REST API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

// NewClient returns a client authenticating with token. It uses
// GITHUB_API_URL, so it also works on GitHub Enterprise Server.
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, errors.New("github token is empty")
	}
	baseURL := os.Getenv("GITHUB_API_URL")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return newClient(baseURL, token), nil
}

func newClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		sleep:   sleep,
	}
}

// APIError is a non-2xx response.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

// IsNotFound reports whether err is a 404 response.
func IsNotFound(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.Status == http.StatusNotFound
}

// do sends a request and decodes the JSON response into out, if not nil. It
// returns the raw response body and the URL of the next page, if any.
func (c *Client) do(ctx context.Context, method, path string, in, out any) ([]byte, string, error) {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return nil, "", err
		}
	}

	url := path
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = c.baseURL + path
	}

	// A failed POST may have taken effect, so only rate limits retry it.
	idempotent := method != http.MethodPost

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "josa42-actions")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.http.Do(req)
		if err != nil {
			if idempotent && attempt < maxAttempts && ctx.Err() == nil {
				if err := c.sleep(ctx, backoff(attempt)); err != nil {
					return nil, "", err
				}
				continue
			}
			return nil, "", err
		}

		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, "", err
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			if out != nil && len(data) > 0 {
				if err := json.Unmarshal(data, out); err != nil {
					return nil, "", fmt.Errorf("%s %s: %w", method, path, err)
				}
			}
			return data, nextPage(res.Header.Get("Link")), nil
		}

		if wait, ok := retryAfter(res, attempt, idempotent); ok && attempt < maxAttempts {
			if err := c.sleep(ctx, wait); err != nil {
				return nil, "", err
			}
			continue
		}

		var msg struct{ Message string }
		json.Unmarshal(data, &msg)
		return nil, "", &APIError{Method: method, Path: path, Status: res.StatusCode, Message: msg.Message}
	}
}

// retryAfter reports whether a failed response should be retried and how long
// to wait first.
func retryAfter(res *http.Response, attempt int, idempotent bool) (time.Duration, bool) {
	if s := res.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			d := time.Duration(n) * time.Second
			return d, d <= maxWait
		}
	}

	rateLimited := res.StatusCode == http.StatusTooManyRequests ||
		(res.StatusCode == http.StatusForbidden && res.Header.Get("X-RateLimit-Remaining") == "0")
	if rateLimited {
		if reset, err := strconv.ParseInt(res.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			d := time.Until(time.Unix(reset, 0)) + time.Second
			return max(d, 0), d <= maxWait
		}
		return backoff(attempt), true
	}

	return backoff(attempt), idempotent && res.StatusCode >= 500
}

func backoff(attempt int) time.Duration {
	return time.Duration(attempt) * time.Second
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

var linkNextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextPage(link string) string {
	if m := linkNextRe.FindStringSubmatch(link); m != nil {
		return m[1]
	}
	return ""
}

func (c *Client) get(ctx context.Context, path string, out any) ([]byte, error) {
	data, _, err := c.do(ctx, http.MethodGet, path, nil, out)
	return data, err
}

// getAll fetches every page of a list endpoint. path must not contain a query.
func getAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	next := path + "?per_page=100"
	for next != "" {
		var page []T
		_, n, err := c.do(ctx, http.MethodGet, next, nil, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		next = n
	}
	return all, nil
}
