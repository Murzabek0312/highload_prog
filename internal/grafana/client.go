package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type (
	Client struct {
		baseURL  string
		user     string
		password string
		token    string
		http     *http.Client
	}

	ServiceAccount struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
)

func New(baseURL string, user string, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		user:     user,
		password: password,
		http: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

func NewWithToken(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) FindServiceAccount(ctx context.Context, name string) (*ServiceAccount, error) {
	address := c.baseURL + "/api/serviceaccounts/search?query=" + url.QueryEscape(name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Accounts []ServiceAccount `json:"serviceAccounts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	for _, account := range result.Accounts {
		if account.Name == name {
			return &account, nil
		}
	}
	return nil, nil
}

func (c *Client) CreateServiceAccount(ctx context.Context, name string) (*ServiceAccount, error) {
	body, err := json.Marshal(map[string]any{
		"name":       name,
		"role":       "Editor",
		"isDisabled": false,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/serviceaccounts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var account ServiceAccount
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}
	return &account, nil

}

func (c *Client) CreateToken(ctx context.Context, accountID int64, name string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"name": name,
	})
	if err != nil {
		return "", err
	}

	address := fmt.Sprintf(
		"%s/api/serviceaccounts/%d/tokens",
		c.baseURL,
		accountID,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		address,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create token: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Key == "" {
		return "", fmt.Errorf("create token: empty key in response")
	}

	return result.Key, nil
}
