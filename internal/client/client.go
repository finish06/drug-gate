package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ErrUpstream indicates the upstream cash-drugs service returned an error or is unreachable.
var ErrUpstream = errors.New("upstream service error")

// DrugResult holds the parsed upstream response for a single drug.
type DrugResult struct {
	ProductNDC  string   `json:"product_ndc"`
	BrandName   string   `json:"brand_name"`
	GenericName string   `json:"generic_name"`
	PharmClass  []string `json:"pharm_class"`
}

// DrugClient defines the interface for looking up drugs.
type DrugClient interface {
	LookupByNDC(ctx context.Context, ndc string) (*DrugResult, error)
}

// HTTPDrugClient queries the cash-drugs API over HTTP.
type HTTPDrugClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPDrugClient creates a client pointing at the given cash-drugs base URL.
func NewHTTPDrugClient(baseURL string) *HTTPDrugClient {
	return &HTTPDrugClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// LookupByNDC queries cash-drugs fda-ndc endpoint with the given product NDC.
// Returns nil (no error) if the drug is not found (404 or empty results).
// Returns ErrUpstream for connection failures or non-200/404 responses.
func (c *HTTPDrugClient) LookupByNDC(ctx context.Context, ndc string) (*DrugResult, error) {
	url := fmt.Sprintf("%s/api/cache/fda-ndc?NDC=%s", c.baseURL, url.QueryEscape(ndc))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		Data []json.RawMessage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	if len(upstream.Data) == 0 {
		return nil, nil
	}

	var result DrugResult
	if err := json.Unmarshal(upstream.Data[0], &result); err != nil {
		return nil, fmt.Errorf("%w: failed to parse drug result: %v", ErrUpstream, err)
	}

	return &result, nil
}
