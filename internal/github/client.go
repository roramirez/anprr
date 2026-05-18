package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
)

const baseURL = "https://api.github.com"

var (
	ErrUnauthorized  = fmt.Errorf("authentication failed: check your token")
	ErrForbidden     = fmt.Errorf("permission denied: token may lack required scopes (repo, read:user)")
	ErrNotFound      = fmt.Errorf("not found")
	ErrUnprocessable = fmt.Errorf("unprocessable: you may be trying to approve your own PR")
	ErrServerError   = fmt.Errorf("GitHub server error")
)

type Client struct {
	token     string
	http      *http.Client
	rateLimit atomic.Int64 // remaining requests
	rateReset atomic.Int64 // unix timestamp of reset
}

func NewClient(token string, transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	c := &Client{
		token: token,
		http:  &http.Client{Transport: transport},
	}
	c.rateLimit.Store(-1)
	return c
}

func (c *Client) RateLimitRemaining() int64 {
	return c.rateLimit.Load()
}

func (c *Client) RateLimitReset() int64 {
	return c.rateReset.Load()
}

func (c *Client) do(method, url string, body interface{}, acceptHeader string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "anprr/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if acceptHeader != "" {
		req.Header.Set("Accept", acceptHeader)
	} else {
		req.Header.Set("Accept", "application/vnd.github.v3+json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// update rate limit counters
	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.rateLimit.Store(n)
		}
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.rateReset.Store(n)
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return data, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusForbidden:
		return nil, ErrForbidden
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnprocessableEntity:
		return nil, ErrUnprocessable
	default:
		if resp.StatusCode >= 500 {
			return nil, ErrServerError
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
}

func (c *Client) REST(method, path string, body interface{}, dst interface{}) error {
	data, err := c.do(method, baseURL+path, body, "")
	if err != nil {
		return err
	}
	if dst != nil {
		return json.Unmarshal(data, dst)
	}
	return nil
}

func (c *Client) GraphQL(query string, variables map[string]interface{}, dst interface{}) error {
	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}
	data, err := c.do("POST", baseURL+"/graphql", payload, "")
	if err != nil {
		return err
	}

	var wrapper struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	if len(wrapper.Errors) > 0 {
		return fmt.Errorf("graphql: %s", wrapper.Errors[0].Message)
	}
	if dst != nil {
		return json.Unmarshal(wrapper.Data, dst)
	}
	return nil
}

type graphqlError struct {
	Message string `json:"message"`
}
