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

// RxNormCandidateRaw is the raw upstream approximate match candidate.
// cash-drugs flattens the RxNorm approximateGroup.candidate array into data[].
type RxNormCandidateRaw struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
	Score string `json:"score"`
}

// RxNormConceptRaw is the raw upstream concept (used in generics, related).
// cash-drugs flattens concept arrays into data[].
type RxNormConceptRaw struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
	TTY   string `json:"tty"`
}

// RxNormConceptGroupRaw is a group of concepts by TTY from allRelated.
// Note: cash-drugs flattens allRelatedGroup.conceptGroup[].conceptProperties
// into a single data[] array with tty on each entry. This type is used
// internally after we re-group by TTY.
type RxNormConceptGroupRaw struct {
	TTY               string             `json:"tty"`
	ConceptProperties []RxNormConceptRaw `json:"conceptProperties"`
}

// RxNormClient defines the interface for RxNorm lookups via cash-drugs.
type RxNormClient interface {
	SearchApproximate(ctx context.Context, name string) ([]RxNormCandidateRaw, error)
	FetchSpellingSuggestions(ctx context.Context, name string) ([]string, error)
	FetchNDCs(ctx context.Context, rxcui string) ([]string, error)
	FetchGenericProduct(ctx context.Context, rxcui string) ([]RxNormConceptRaw, error)
	FetchAllRelated(ctx context.Context, rxcui string) ([]RxNormConceptGroupRaw, error)
}

// HTTPRxNormClient queries cash-drugs RxNorm proxy endpoints.
type HTTPRxNormClient struct {
	baseURL    string
	httpClient *http.Client
	breaker    *CircuitBreaker
}

// NewHTTPRxNormClient creates a client pointing at the given cash-drugs base URL.
func NewHTTPRxNormClient(baseURL string, breaker ...*CircuitBreaker) *HTTPRxNormClient {
	var cb *CircuitBreaker
	if len(breaker) > 0 {
		cb = breaker[0]
	} else {
		cb = NewCircuitBreaker(10, 30*time.Second)
	}
	return &HTTPRxNormClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		breaker: cb,
	}
}

// doRequest wraps HTTP calls with circuit breaker and response size limiting.
func (c *HTTPRxNormClient) doRequest(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	err := c.breaker.Execute(func() error {
		var doErr error
		resp, doErr = c.httpClient.Do(req)
		if doErr != nil {
			return doErr
		}
		resp.Body = &limitedReadCloser{Reader: io.LimitReader(resp.Body, maxResponseBytes), Closer: resp.Body}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("upstream returned status %d", resp.StatusCode)
		}
		return nil
	})
	if err != nil && resp != nil && resp.StatusCode < 500 {
		return resp, nil
	}
	return resp, err
}

func (c *HTTPRxNormClient) doGet(ctx context.Context, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	return resp, nil
}

// SearchApproximate searches for drugs by approximate name match.
// cash-drugs returns: {"data": [{rxcui, name, score, ...}, ...]}
func (c *HTTPRxNormClient) SearchApproximate(ctx context.Context, name string) ([]RxNormCandidateRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-approximate-match?DRUG_NAME=%s", c.baseURL, url.QueryEscape(name))

	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var upstream struct {
		Data []RxNormCandidateRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}

// FetchSpellingSuggestions fetches spelling suggestions for a drug name.
// cash-drugs returns: {"data": [{"suggestionGroup": {"suggestionList": {"suggestion": [...]}}}]}
func (c *HTTPRxNormClient) FetchSpellingSuggestions(ctx context.Context, name string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-spelling-suggestions?DRUG_NAME=%s", c.baseURL, url.QueryEscape(name))

	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var upstream struct {
		Data []struct {
			SuggestionGroup struct {
				SuggestionList struct {
					Suggestion []string `json:"suggestion"`
				} `json:"suggestionList"`
			} `json:"suggestionGroup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	if len(upstream.Data) > 0 {
		return upstream.Data[0].SuggestionGroup.SuggestionList.Suggestion, nil
	}
	return nil, nil
}

// FetchNDCs fetches NDC codes for the given RxCUI.
// cash-drugs returns: {"data": [{"ndcGroup": {"ndcList": {"ndc": [...]}}}]}
func (c *HTTPRxNormClient) FetchNDCs(ctx context.Context, rxcui string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-ndcs?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var upstream struct {
		Data []struct {
			NDCGroup struct {
				NDCList struct {
					NDC []string `json:"ndc"`
				} `json:"ndcList"`
			} `json:"ndcGroup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	if len(upstream.Data) > 0 {
		return upstream.Data[0].NDCGroup.NDCList.NDC, nil
	}
	return nil, nil
}

// FetchGenericProduct fetches generic product concepts for the given RxCUI.
// cash-drugs returns: {"data": [{rxcui, name, tty}, ...]}
func (c *HTTPRxNormClient) FetchGenericProduct(ctx context.Context, rxcui string) ([]RxNormConceptRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-generic-product?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var upstream struct {
		Data []RxNormConceptRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data, nil
}

// FetchAllRelated fetches all related concept groups for the given RxCUI.
// cash-drugs returns: {"data": [{rxcui, name, tty, ...}, ...]} — flat array
// with tty on each entry. We re-group by TTY before returning.
func (c *HTTPRxNormClient) FetchAllRelated(ctx context.Context, rxcui string) ([]RxNormConceptGroupRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-all-related?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	resp, err := c.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var upstream struct {
		Data []RxNormConceptRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	// Re-group flat entries by TTY
	groupMap := make(map[string][]RxNormConceptRaw)
	for _, entry := range upstream.Data {
		groupMap[entry.TTY] = append(groupMap[entry.TTY], entry)
	}

	groups := make([]RxNormConceptGroupRaw, 0, len(groupMap))
	for tty, concepts := range groupMap {
		groups = append(groups, RxNormConceptGroupRaw{
			TTY:               tty,
			ConceptProperties: concepts,
		})
	}

	return groups, nil
}
