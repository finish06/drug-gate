package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// maxResponseBytes is the maximum upstream response body size (5MB).
const maxResponseBytes = 5 << 20

// limitedReadCloser wraps a LimitReader while preserving the original body's
// Close method for HTTP connection reuse.
type limitedReadCloser struct {
	io.Reader
	io.Closer
}

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
	LookupByGenericName(ctx context.Context, name string) ([]DrugResult, error)
	LookupByBrandName(ctx context.Context, name string) ([]DrugResult, error)
	FetchDrugNames(ctx context.Context) ([]DrugNameRaw, error)
	FetchDrugClasses(ctx context.Context) ([]DrugClassRaw, error)
	LookupByPharmClass(ctx context.Context, class string) ([]DrugResult, error)
}

// DrugNameRaw is the raw upstream drug name entry from cash-drugs.
type DrugNameRaw struct {
	NameType string `json:"name_type"`
	DrugName string `json:"drug_name"`
}

// DrugClassRaw is the raw upstream drug class entry from cash-drugs.
type DrugClassRaw struct {
	ClassName string `json:"name"`
	ClassType string `json:"type"`
}

// HTTPDrugClient queries the cash-drugs API over HTTP.
type HTTPDrugClient struct {
	baseURL    string
	httpClient *http.Client
	breaker    *CircuitBreaker
}

// NewHTTPDrugClient creates a client pointing at the given cash-drugs base URL.
func NewHTTPDrugClient(baseURL string, breaker ...*CircuitBreaker) *HTTPDrugClient {
	var cb *CircuitBreaker
	if len(breaker) > 0 {
		cb = breaker[0]
	} else {
		cb = NewCircuitBreaker(10, 30*time.Second)
	}
	return &HTTPDrugClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		breaker: cb,
	}
}

// Breaker returns the circuit breaker for health check inspection.
func (c *HTTPDrugClient) Breaker() *CircuitBreaker {
	return c.breaker
}

// doRequest executes an HTTP request wrapped with circuit breaker protection
// and response body size limiting.
func (c *HTTPDrugClient) doRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := c.breaker.Execute(func() error {
		var doErr error
		resp, doErr = c.httpClient.Do(req)
		if doErr != nil {
			return doErr
		}
		// Limit response body size to 5MB (preserve original Close for connection reuse)
		resp.Body = &limitedReadCloser{Reader: io.LimitReader(resp.Body, maxResponseBytes), Closer: resp.Body}
		// Count 5xx as upstream failures (trips breaker)
		// 4xx are client errors — don't trip the breaker
		if resp.StatusCode >= 500 {
			return fmt.Errorf("upstream returned status %d", resp.StatusCode)
		}
		return nil
	})
	if err != nil && resp != nil && resp.StatusCode < 500 {
		// Non-5xx response with breaker wrapping — return response for status handling
		return resp, nil
	}
	return resp, err
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

	resp, err := c.doRequest(req)
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

// lookupByNameParam queries cash-drugs fda-ndc with a name-based search param.
func (c *HTTPDrugClient) lookupByNameParam(ctx context.Context, param, value string) ([]DrugResult, error) {
	reqURL := fmt.Sprintf("%s/api/cache/fda-ndc?%s=%s", c.baseURL, param, url.QueryEscape(value))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.doRequest(req)
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
		Data []DrugResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}

// LookupByGenericName queries cash-drugs fda-ndc by GENERIC_NAME.
func (c *HTTPDrugClient) LookupByGenericName(ctx context.Context, name string) ([]DrugResult, error) {
	return c.lookupByNameParam(ctx, "GENERIC_NAME", name)
}

// LookupByBrandName queries cash-drugs fda-ndc by BRAND_NAME.
func (c *HTTPDrugClient) LookupByBrandName(ctx context.Context, name string) ([]DrugResult, error) {
	return c.lookupByNameParam(ctx, "BRAND_NAME", name)
}

// LookupByPharmClass queries cash-drugs fda-ndc by PHARM_CLASS.
func (c *HTTPDrugClient) LookupByPharmClass(ctx context.Context, class string) ([]DrugResult, error) {
	return c.lookupByNameParam(ctx, "PHARM_CLASS", class)
}

// FetchDrugNames fetches the full drug names dataset from cash-drugs.
func (c *HTTPDrugClient) FetchDrugNames(ctx context.Context) ([]DrugNameRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/drugnames", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data []DrugNameRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}

// FetchDrugClasses fetches the full drug classes dataset from cash-drugs.
func (c *HTTPDrugClient) FetchDrugClasses(ctx context.Context) ([]DrugClassRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/drugclasses", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data []DrugClassRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}
