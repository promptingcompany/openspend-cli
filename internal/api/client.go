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
	"strings"
	"time"
)

type Options struct {
	BaseURL         string
	SessionToken    string
	SessionCookie   string
	WhoAmIPath      string
	PolicyInitPath  string
	AgentPath       string
	BrowserAuthPath string
}

type Client struct {
	baseURL       string
	httpClient    *http.Client
	sessionToken  string
	sessionCookie string
	whoAmIPath    string
	policyPath    string
	agentPath     string
	authPath      string
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

func New(opts Options) *Client {
	return &Client{
		baseURL:       strings.TrimRight(opts.BaseURL, "/"),
		httpClient:    &http.Client{Timeout: 20 * time.Second},
		sessionToken:  opts.SessionToken,
		sessionCookie: fallback(opts.SessionCookie, "better-auth.session_token"),
		whoAmIPath:    fallback(opts.WhoAmIPath, "/api/cli/whoami"),
		policyPath:    fallback(opts.PolicyInitPath, "/api/cli/policy/init"),
		agentPath:     fallback(opts.AgentPath, "/api/cli/agent"),
		authPath:      fallback(opts.BrowserAuthPath, "/api/cli/auth/login"),
	}
}

func (c *Client) SetSessionToken(token string) {
	c.sessionToken = token
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

func (c *Client) do(ctx context.Context, method, path string, body any, withSession bool) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if withSession {
		if c.sessionToken == "" {
			return nil, errors.New("not authenticated; run openspend auth login")
		}
		req.AddCookie(&http.Cookie{Name: c.sessionCookie, Value: c.sessionToken})
	}

	return c.httpClient.Do(req)
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
