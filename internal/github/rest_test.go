package github

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// mockTransport records the last request and returns a configured response.
type mockTransport struct {
	statusCode  int
	body        string
	lastReq     *http.Request
	lastReqBody []byte
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if req.Body != nil {
		m.lastReqBody, _ = io.ReadAll(req.Body)
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(statusCode int, body string) (*Client, *mockTransport) {
	mt := &mockTransport{statusCode: statusCode, body: body}
	return NewClient("test-token", mt), mt
}

func TestGetCurrentUser(t *testing.T) {
	c, mt := newTestClient(200, `{"login":"jdoe"}`)
	login, err := c.GetCurrentUser()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "jdoe" {
		t.Errorf("got %q", login)
	}
	if mt.lastReq.Header.Get("Authorization") != "Bearer test-token" {
		t.Error("missing auth header")
	}
}

func TestGetDiff_acceptHeader(t *testing.T) {
	c, mt := newTestClient(200, "diff content")
	cache := NewCache()
	diff, err := c.GetDiff("owner", "repo", 42, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff != "diff content" {
		t.Errorf("got %q", diff)
	}
	if mt.lastReq.Header.Get("Accept") != "application/vnd.github.v3.diff" {
		t.Errorf("wrong Accept header: %q", mt.lastReq.Header.Get("Accept"))
	}
}

func TestGetDiff_caching(t *testing.T) {
	c, mt := newTestClient(200, "diff content")
	cache := NewCache()
	c.GetDiff("owner", "repo", 42, cache)
	c.GetDiff("owner", "repo", 42, cache)
	// second call should use cache, so only 1 HTTP request
	_ = mt // mt.lastReq is still set from first call
	if v, ok := cache.Get("diff:owner/repo#42"); !ok || v.(string) != "diff content" {
		t.Error("expected cached diff")
	}
}

func TestSubmitReview_approve(t *testing.T) {
	c, mt := newTestClient(200, `{}`)
	err := c.SubmitReview("owner", "repo", 42, ReviewApprove, "looks good", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]interface{}
	json.Unmarshal(mt.lastReqBody, &body)
	if body["event"] != "APPROVE" {
		t.Errorf("event: got %q", body["event"])
	}
	if body["body"] != "looks good" {
		t.Errorf("body: got %q", body["body"])
	}
}

func TestSubmitReview_requestChanges(t *testing.T) {
	c, mt := newTestClient(200, `{}`)
	c.SubmitReview("owner", "repo", 1, ReviewRequestChanges, "needs work", nil)
	var body map[string]interface{}
	json.Unmarshal(mt.lastReqBody, &body)
	if body["event"] != "REQUEST_CHANGES" {
		t.Errorf("got %q", body["event"])
	}
}

func TestSubmitReview_withInlineComments(t *testing.T) {
	c, mt := newTestClient(200, `{}`)
	inline := []InlineComment{
		{Path: "auth/token.go", Line: 42, Side: "RIGHT", Body: "this needs nil check"},
		{Path: "auth/token.go", Line: 10, Side: "LEFT", Body: "removed incorrectly"},
	}
	err := c.SubmitReview("owner", "repo", 7, ReviewRequestChanges, "see inline", inline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]interface{}
	json.Unmarshal(mt.lastReqBody, &body)

	comments, ok := body["comments"].([]interface{})
	if !ok || len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %v", body["comments"])
	}
	first := comments[0].(map[string]interface{})
	if first["path"] != "auth/token.go" {
		t.Errorf("path: got %q", first["path"])
	}
	if first["side"] != "RIGHT" {
		t.Errorf("side: got %q", first["side"])
	}
	if first["body"] != "this needs nil check" {
		t.Errorf("body: got %q", first["body"])
	}
}

func TestPostComment(t *testing.T) {
	c, mt := newTestClient(201, `{"id":1}`)
	err := c.PostComment("owner", "repo", 42, "great PR!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]string
	json.Unmarshal(mt.lastReqBody, &body)
	if body["body"] != "great PR!" {
		t.Errorf("got %q", body["body"])
	}
}

func TestHTTP403_returnsErrForbidden(t *testing.T) {
	c, _ := newTestClient(403, `{"message":"Forbidden"}`)
	_, err := c.GetCurrentUser()
	if err != ErrForbidden {
		t.Errorf("got %v", err)
	}
}

func TestHTTP422_returnsErrUnprocessable(t *testing.T) {
	c, _ := newTestClient(422, `{"message":"Unprocessable"}`)
	err := c.SubmitReview("o", "r", 1, ReviewApprove, "", nil)
	if err != ErrUnprocessable {
		t.Errorf("got %v", err)
	}
}

func TestHTTP500_returnsErrServerError(t *testing.T) {
	c, _ := newTestClient(500, `{"message":"Internal Server Error"}`)
	_, err := c.GetCurrentUser()
	if err != ErrServerError {
		t.Errorf("got %v", err)
	}
}

func TestHTTP401_returnsErrUnauthorized(t *testing.T) {
	c, _ := newTestClient(401, `{"message":"Bad credentials"}`)
	_, err := c.GetCurrentUser()
	if err != ErrUnauthorized {
		t.Errorf("got %v", err)
	}
}

func TestHTTP404_returnsErrNotFound(t *testing.T) {
	c, _ := newTestClient(404, `{"message":"Not Found"}`)
	_, err := c.GetCurrentUser()
	if err != ErrNotFound {
		t.Errorf("got %v", err)
	}
}

func TestRateLimitHeader(t *testing.T) {
	mt := &mockTransport{
		statusCode: 200,
		body:       `{"login":"u"}`,
	}
	mt2 := &rateLimitTransport{inner: mt, remaining: "42", reset: "9999"}
	c := NewClient("tok", mt2)
	c.GetCurrentUser()
	if c.RateLimitRemaining() != 42 {
		t.Errorf("got %d", c.RateLimitRemaining())
	}
}

func TestMergePR_squash(t *testing.T) {
	c, mt := newTestClient(200, `{"sha":"abc","merged":true,"message":"squashed"}`)
	err := c.MergePR("owner", "repo", 42, MergeMethodSquash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mt.lastReq.Method != "PUT" {
		t.Errorf("expected PUT, got %s", mt.lastReq.Method)
	}
	if !strings.Contains(mt.lastReq.URL.Path, "/pulls/42/merge") {
		t.Errorf("unexpected path: %s", mt.lastReq.URL.Path)
	}
	var body map[string]string
	json.Unmarshal(mt.lastReqBody, &body)
	if body["merge_method"] != "squash" {
		t.Errorf("expected squash, got %q", body["merge_method"])
	}
}

func TestMergePR_rebase(t *testing.T) {
	c, mt := newTestClient(200, `{"sha":"def","merged":true}`)
	c.MergePR("owner", "repo", 7, MergeMethodRebase)
	var body map[string]string
	json.Unmarshal(mt.lastReqBody, &body)
	if body["merge_method"] != "rebase" {
		t.Errorf("expected rebase, got %q", body["merge_method"])
	}
}

// FetchComments uses two consecutive requests (issues comments then pull comments).
// We use a counter-based transport to serve different responses per call.

type multiTransport struct {
	responses []struct {
		status int
		body   string
	}
	call int
}

func (m *multiTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := m.responses[m.call%len(m.responses)]
	m.call++
	return &http.Response{
		StatusCode: r.status,
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     make(http.Header),
	}, nil
}

func TestFetchComments_parsesResults(t *testing.T) {
	commentsJSON := `[{"body":"looks good","created_at":"2026-05-18T10:00:00Z","user":{"login":"alice"}}]`
	reviewJSON := `[{"body":"nil check needed","path":"auth/token.go","line":42,"position":5,"user":{"login":"bob"}}]`
	mt := &multiTransport{responses: []struct {
		status int
		body   string
	}{{200, commentsJSON}, {200, reviewJSON}}}
	c := NewClient("tok", mt)

	comments, lineComments, err := c.FetchComments("owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 || comments[0].Author.Login != "alice" || comments[0].Body != "looks good" {
		t.Errorf("comments: %+v", comments)
	}
	if len(lineComments) != 1 || lineComments[0].Author.Login != "bob" ||
		lineComments[0].Path != "auth/token.go" || lineComments[0].Line != 42 {
		t.Errorf("lineComments: %+v", lineComments)
	}
}

func TestFetchComments_emptyResponse(t *testing.T) {
	mt := &multiTransport{responses: []struct {
		status int
		body   string
	}{{200, "[]"}, {200, "[]"}}}
	c := NewClient("tok", mt)

	comments, lineComments, err := c.FetchComments("owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 0 || len(lineComments) != 0 {
		t.Errorf("expected empty results")
	}
}

func TestFetchComments_errorOnFirstRequest(t *testing.T) {
	mt := &multiTransport{responses: []struct {
		status int
		body   string
	}{{404, `{"message":"not found"}`}, {200, "[]"}}}
	c := NewClient("tok", mt)

	_, _, err := c.FetchComments("owner", "repo", 1)
	if err == nil {
		t.Error("expected error on 404")
	}
}

func TestSearchReviewRequested_parsesResults(t *testing.T) {
	body := `{"total_count":2,"items":[
		{"number":42,"repository_url":"https://api.github.com/repos/myorg/backend"},
		{"number":7,"repository_url":"https://api.github.com/repos/myorg/frontend"}
	]}`
	c, mt := newTestClient(200, body)
	cache := NewCache()
	set, err := c.SearchReviewRequested([]string{"myorg/backend", "myorg/frontend"}, cache)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !set["myorg/backend#42"] {
		t.Error("expected myorg/backend#42 in set")
	}
	if !set["myorg/frontend#7"] {
		t.Error("expected myorg/frontend#7 in set")
	}
	// query should contain review-requested:@me
	reqURL := mt.lastReq.URL.RawQuery
	if !strings.Contains(reqURL, "review-requested") {
		t.Errorf("query missing review-requested: %s", reqURL)
	}
}

func TestSearchReviewRequested_caching(t *testing.T) {
	body := `{"total_count":1,"items":[{"number":1,"repository_url":"https://api.github.com/repos/a/b"}]}`
	c, _ := newTestClient(200, body)
	cache := NewCache()
	set1, _ := c.SearchReviewRequested([]string{"a/b"}, cache)
	set2, _ := c.SearchReviewRequested([]string{"a/b"}, cache)
	// second call should return cached result
	if len(set1) != len(set2) {
		t.Error("expected same result from cache")
	}
	if _, ok := cache.Get(reviewRequestedKey); !ok {
		t.Error("expected result in cache")
	}
}

func TestSearchReviewRequested_buildsRepoFilters(t *testing.T) {
	body := `{"total_count":0,"items":[]}`
	c, mt := newTestClient(200, body)
	cache := NewCache()
	c.SearchReviewRequested([]string{"org/repo-a", "org/repo-b"}, cache)
	rawQuery, _ := url.QueryUnescape(mt.lastReq.URL.RawQuery)
	if !strings.Contains(rawQuery, "repo:org/repo-a") {
		t.Errorf("query missing repo filter: %s", rawQuery)
	}
	if !strings.Contains(rawQuery, "repo:org/repo-b") {
		t.Errorf("query missing repo filter: %s", rawQuery)
	}
}

// rateLimitTransport wraps a transport and injects rate-limit headers.
type rateLimitTransport struct {
	inner     http.RoundTripper
	remaining string
	reset     string
}

func (r *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Header.Set("X-RateLimit-Remaining", r.remaining)
	resp.Header.Set("X-RateLimit-Reset", r.reset)
	return resp, nil
}
