package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SPLEntryRaw is the raw upstream SPL metadata entry from cash-drugs.
type SPLEntryRaw struct {
	Title         string `json:"title"`
	SetID         string `json:"setid"`
	PublishedDate string `json:"published_date"`
	SPLVersion    int    `json:"spl_version"`
}

// SPLClient defines the interface for SPL lookups via cash-drugs.
type SPLClient interface {
	FetchSPLsByName(ctx context.Context, drugName string) ([]SPLEntryRaw, error)
	FetchSPLDetail(ctx context.Context, setID string) (*SPLEntryRaw, error)
	FetchSPLXML(ctx context.Context, setID string) ([]byte, error)
}

// HTTPSPLClient queries cash-drugs SPL proxy endpoints.
type HTTPSPLClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPSPLClient creates a client pointing at the given cash-drugs base URL.
func NewHTTPSPLClient(baseURL string) *HTTPSPLClient {
	return &HTTPSPLClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // SPL XML can be large
		},
	}
}

// FetchSPLsByName queries cash-drugs spls-by-name endpoint.
func (c *HTTPSPLClient) FetchSPLsByName(ctx context.Context, drugName string) ([]SPLEntryRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/spls-by-name?DRUGNAME=%s", c.baseURL, url.QueryEscape(drugName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data []SPLEntryRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}

// FetchSPLDetail queries cash-drugs spl-detail endpoint by set ID.
func (c *HTTPSPLClient) FetchSPLDetail(ctx context.Context, setID string) (*SPLEntryRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/spl-detail?SETID=%s", c.baseURL, url.QueryEscape(setID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data []SPLEntryRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	if len(upstream.Data) == 0 {
		return nil, nil
	}

	return &upstream.Data[0], nil
}

// FetchSPLXML fetches the raw SPL XML document from cash-drugs.
func (c *HTTPSPLClient) FetchSPLXML(ctx context.Context, setID string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/api/cache/spl-xml?SETID=%s", c.baseURL, url.QueryEscape(setID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read XML body: %v", ErrUpstream, err)
	}

	return data, nil
}
