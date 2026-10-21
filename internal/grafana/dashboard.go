package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/resource"
)

const dashboardsAPIPath = "/apis/dashboard.grafana.app/v2/namespaces/default/dashboards"

func (c *Client) GetDashboardMetadata(ctx context.Context, uid string) (*resource.Metadata, error) {
	address := c.baseURL + dashboardsAPIPath + "/" + url.PathEscape(uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get dashboard: HTTP %d", resp.StatusCode)
	}

	var result struct {
		Metadata resource.Metadata `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode dashboard metadata: %w", err)
	}

	return &result.Metadata, nil
}

func (c *Client) SaveDashboard(ctx context.Context, uid string, board dashboardv2.Dashboard) error {
	metadata, err := c.GetDashboardMetadata(ctx, uid)
	if err != nil {
		return err
	}

	method := http.MethodPost
	address := c.baseURL + dashboardsAPIPath
	if metadata == nil {
		metadata = &resource.Metadata{Name: uid}
	} else {
		method = http.MethodPut
		address += "/" + url.PathEscape(uid)
	}

	manifest := resource.Manifest{
		ApiVersion: "dashboard.grafana.app/v2",
		Kind:       "Dashboard",
		Metadata:   *metadata,
		Spec:       board,
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode dashboard: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("save dashboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("save dashboard: HTTP %d", resp.StatusCode)
	}

	return nil
}
