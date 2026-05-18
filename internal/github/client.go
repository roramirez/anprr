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
	req, err := c.buildRequest(method, url, body, acceptHeader)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	c.updateRateLimit(resp)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, checkStatus(resp.StatusCode, data)
}

func (c *Client) buildRequest(method, url string, body interface{}, acceptHeader string) (*http.Request, error) {
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
	accept := "application/vnd.github.v3+json"
	if acceptHeader != "" {
		accept = acceptHeader
	}
	req.Header.Set("Accept", accept)
	return req, nil
}

func (c *Client) updateRateLimit(resp *http.Response) {
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
}

func checkStatus(code int, data []byte) error {
	switch code {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnprocessableEntity:
		return ErrUnprocessable
	default:
		if code >= 500 {
			return ErrServerError
		}
		return fmt.Errorf("HTTP %d: %s", code, string(data))
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
