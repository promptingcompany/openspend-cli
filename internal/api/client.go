package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errSessionExpired = errors.New("session expired; run openspend auth login")

type Options struct {
	BaseURL            string
	SessionToken       string
	SessionCookie      string
	SessionExpiresAt   time.Time
	WhoAmIPath         string
	PolicyInitPath     string
	AgentPath          string
	SearchPath         string
	BrowserAuthPath    string
	SessionRefreshPath string
}

type Client struct {
	baseURL            string
	httpClient         *http.Client
	sessionToken       string
	sessionCookie      string
	sessionExpiresAt   time.Time
	whoAmIPath         string
	policyPath         string
	agentPath          string
	searchPath         string
	authPath           string
	sessionRefreshPath string
}

type WhoAmIResponse struct {
	User struct {
		ID    string  `json:"id"`
		Email *string `json:"email"`
		Name  *string `json:"name"`
	} `json:"user"`
	Subjects []struct {
		ID          string  `json:"id"`
		Kind        string  `json:"kind"`
		ExternalKey *string `json:"externalKey"`
		DisplayName *string `json:"displayName"`
		Status      string  `json:"status"`
		PolicyID    *string `json:"policyId"`
		PolicyName  *string `json:"policyName"`
		PolicyMode  *string `json:"policyMode"`
		Precedence  *int    `json:"precedence"`
	} `json:"subjects"`
}

type InitPolicyRequest struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	MaxPrice    *int64   `json:"maxPrice,omitempty"`
	Asset       string   `json:"asset,omitempty"`
	Network     string   `json:"network,omitempty"`
	DenyHosts   []string `json:"denyHosts,omitempty"`
}

type InitPolicyResponse struct {
	Policy struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"policy"`
	Created bool `json:"created"`
}

type CreateAgentRequest struct {
	ExternalKey string `json:"externalKey"`
	DisplayName string `json:"displayName,omitempty"`
	Kind        string `json:"kind,omitempty"`
	PolicyID    string `json:"policyId,omitempty"`
}

type CreateAgentResponse struct {
	Subject struct {
		ID          string `json:"id"`
		ExternalKey string `json:"externalKey"`
		DisplayName string `json:"displayName"`
		Kind        string `json:"kind"`
	} `json:"subject"`
	PolicyID string `json:"policyId"`
	Bound    bool   `json:"bound"`
}

type SearchRequest struct {
	Query            string
	Networks         []string
	Limit            int
	BudgetMax        *float64
	BudgetAsset      string
	MinServiceScore  *float64
	MinProviderScore *float64
	MinPaymentScore  *float64
}

type SearchResponse struct {
	Items      []SearchResultItem `json:"items"`
	Pagination struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination"`
}

type SearchResultItem struct {
	ID          string   `json:"id"`
	ResourceURL string   `json:"resourceUrl"`
	Type        string   `json:"type"`
	Networks    []string `json:"networks"`
	Description string   `json:"description"`
	MinPrice    float64  `json:"minPrice"`
	Asset       string   `json:"asset"`
	Origin      struct {
		URL     string  `json:"url"`
		Title   *string `json:"title"`
		Favicon *string `json:"favicon"`
	} `json:"origin"`
	Score float64 `json:"score"`
}

func New(opts Options) *Client {
	return &Client{
		baseURL:            strings.TrimRight(opts.BaseURL, "/"),
		httpClient:         &http.Client{Timeout: 20 * time.Second},
		sessionToken:       opts.SessionToken,
		sessionCookie:      fallback(opts.SessionCookie, "better-auth.session_token"),
		sessionExpiresAt:   opts.SessionExpiresAt,
		whoAmIPath:         fallback(opts.WhoAmIPath, "/api/cli/whoami"),
		policyPath:         fallback(opts.PolicyInitPath, "/api/cli/policy/init"),
		agentPath:          fallback(opts.AgentPath, "/api/cli/agent"),
		searchPath:         fallback(opts.SearchPath, "/api/search"),
		authPath:           fallback(opts.BrowserAuthPath, "/api/cli/auth/login"),
		sessionRefreshPath: fallback(opts.SessionRefreshPath, "/api/auth/get-session"),
	}
}

func (c *Client) SetSessionToken(token string) {
	c.sessionToken = token
}

func (c *Client) SessionToken() string {
	return c.sessionToken
}

func (c *Client) SessionCookie() string {
	return c.sessionCookie
}

func (c *Client) SessionExpiresAt() time.Time {
	return c.sessionExpiresAt
}

func (c *Client) SyncSession(ctx context.Context) error {
	return c.refreshSession(ctx, true)
}

func (c *Client) BrowserLoginURL(callbackURL string) (string, error) {
	if strings.TrimSpace(callbackURL) == "" {
		return "", errors.New("callback URL is required")
	}
	u, err := url.Parse(c.baseURL + c.authPath)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("redirect_uri", callbackURL)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) WhoAmI(ctx context.Context) (WhoAmIResponse, error) {
	res, err := c.do(ctx, http.MethodGet, c.whoAmIPath, nil, true)
	if err != nil {
		return WhoAmIResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return WhoAmIResponse{}, fmt.Errorf("whoami failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out WhoAmIResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return WhoAmIResponse{}, err
	}
	return out, nil
}

func (c *Client) InitPolicy(ctx context.Context, req InitPolicyRequest) (InitPolicyResponse, error) {
	res, err := c.do(ctx, http.MethodPost, c.policyPath, req, true)
	if err != nil {
		return InitPolicyResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return InitPolicyResponse{}, fmt.Errorf("policy init failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out InitPolicyResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return InitPolicyResponse{}, err
	}
	return out, nil
}

func (c *Client) CreateAgent(ctx context.Context, req CreateAgentRequest) (CreateAgentResponse, error) {
	res, err := c.do(ctx, http.MethodPost, c.agentPath, req, true)
	if err != nil {
		return CreateAgentResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return CreateAgentResponse{}, fmt.Errorf("agent create failed: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out CreateAgentResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return CreateAgentResponse{}, err
	}
	return out, nil
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	if strings.TrimSpace(req.Query) == "" {
		return SearchResponse{}, errors.New("query is required")
	}

	params := url.Values{}
	params.Set("q", req.Query)

	if req.Limit > 0 {
		params.Set("limit", strconv.Itoa(req.Limit))
	}
	for _, network := range req.Networks {
		network = strings.TrimSpace(network)
		if network == "" {
			continue
		}
		params.Add("network", network)
	}
	if req.BudgetMax != nil {
		params.Set("budgetMax", strconv.FormatFloat(*req.BudgetMax, 'f', -1, 64))
	}
	if strings.TrimSpace(req.BudgetAsset) != "" {
		params.Set("budgetAsset", strings.TrimSpace(req.BudgetAsset))
	}
	if req.MinServiceScore != nil {
		params.Set("minServiceScore", strconv.FormatFloat(*req.MinServiceScore, 'f', -1, 64))
	}
	if req.MinProviderScore != nil {
		params.Set("minProviderScore", strconv.FormatFloat(*req.MinProviderScore, 'f', -1, 64))
	}
	if req.MinPaymentScore != nil {
		params.Set("minPaymentScore", strconv.FormatFloat(*req.MinPaymentScore, 'f', -1, 64))
	}

	searchPath := c.searchPath
	if encoded := params.Encode(); encoded != "" {
		searchPath += "?" + encoded
	}

	res, err := c.do(ctx, http.MethodGet, searchPath, nil, false)
	if err != nil {
		return SearchResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return SearchResponse{}, fmt.Errorf(
			"search failed: status=%d body=%s",
			res.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var out SearchResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return SearchResponse{}, err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, withSession bool) (*http.Response, error) {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = encoded
	}

	if withSession {
		if err := c.ensureSession(ctx); err != nil {
			return nil, err
		}
	}

	res, err := c.doRequest(ctx, method, path, payload, withSession)
	if err != nil {
		return nil, err
	}
	if withSession {
		c.captureSessionCookie(res)
	}

	if withSession && res.StatusCode == http.StatusUnauthorized {
		_ = res.Body.Close()
		if err := c.refreshSession(ctx, true); err != nil {
			return nil, err
		}
		retryRes, err := c.doRequest(ctx, method, path, payload, withSession)
		if err != nil {
			return nil, err
		}
		c.captureSessionCookie(retryRes)
		return retryRes, nil
	}

	return res, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, payload []byte, withSession bool) (*http.Response, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if withSession {
		for _, cookieName := range c.sessionCookieCandidates() {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: c.sessionToken})
		}
	}

	return c.httpClient.Do(req)
}

func (c *Client) ensureSession(ctx context.Context) error {
	if c.sessionToken == "" {
		return errors.New("not authenticated; run openspend auth login")
	}

	now := time.Now()
	if !c.sessionExpiresAt.IsZero() {
		if !now.Before(c.sessionExpiresAt) {
			return errSessionExpired
		}
		// Proactively refresh shortly before expiry so long-lived CLI sessions stay valid.
		if now.Add(2 * time.Minute).After(c.sessionExpiresAt) {
			if err := c.refreshSession(ctx, false); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Client) refreshSession(ctx context.Context, force bool) error {
	res, err := c.doRequest(ctx, http.MethodGet, c.sessionRefreshPath, nil, true)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	c.captureSessionCookie(res)
	if c.sessionToken == "" {
		return errSessionExpired
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return errSessionExpired
	}
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf(
			"session refresh failed: status=%d body=%s",
			res.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var payload struct {
		Session *struct {
			ExpiresAt *time.Time `json:"expiresAt"`
		} `json:"session"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		if force {
			return err
		}
		// Avoid hard failures for non-JSON responses as long as request succeeded and token remains valid.
		return nil
	}
	if payload.Session == nil {
		return errSessionExpired
	}
	if payload.Session.ExpiresAt != nil {
		c.sessionExpiresAt = payload.Session.ExpiresAt.UTC()
	}
	return nil
}

func (c *Client) captureSessionCookie(res *http.Response) {
	for _, cookie := range res.Cookies() {
		if !isSessionCookieName(cookie.Name) {
			continue
		}
		if cookie.MaxAge < 0 {
			c.sessionToken = ""
			c.sessionExpiresAt = time.Time{}
			return
		}
		if cookie.Value == "" {
			continue
		}
		c.sessionCookie = cookie.Name
		c.sessionToken = cookie.Value
		switch {
		case !cookie.Expires.IsZero():
			c.sessionExpiresAt = cookie.Expires.UTC()
		case cookie.MaxAge > 0:
			c.sessionExpiresAt = time.Now().Add(time.Duration(cookie.MaxAge) * time.Second).UTC()
		}
	}
}

func isSessionCookieName(name string) bool {
	for _, candidate := range sessionCookieCandidates() {
		if name == candidate {
			return true
		}
	}
	return false
}

func (c *Client) sessionCookieCandidates() []string {
	ordered := make([]string, 0, len(sessionCookieCandidates())+1)
	seen := make(map[string]struct{}, len(sessionCookieCandidates())+1)

	if c.sessionCookie != "" {
		ordered = append(ordered, c.sessionCookie)
		seen[c.sessionCookie] = struct{}{}
	}
	for _, candidate := range sessionCookieCandidates() {
		if _, exists := seen[candidate]; exists {
			continue
		}
		ordered = append(ordered, candidate)
		seen[candidate] = struct{}{}
	}
	return ordered
}

func sessionCookieCandidates() []string {
	return []string{
		"better-auth.session_token",
		"better-auth-session_token",
		"__Secure-better-auth.session_token",
		"__Secure-better-auth-session_token",
		"__Host-better-auth.session_token",
		"__Host-better-auth-session_token",
	}
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
