package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// RxNormCandidateRaw is the raw upstream approximate match candidate.
type RxNormCandidateRaw struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
	Score string `json:"score"`
}

// RxNormConceptRaw is the raw upstream concept (used in generics, related).
type RxNormConceptRaw struct {
	RxCUI string `json:"rxcui"`
	Name  string `json:"name"`
}

// RxNormConceptGroupRaw is a group of concepts by TTY from allRelated.
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
}

// NewHTTPRxNormClient creates a client pointing at the given cash-drugs base URL.
func NewHTTPRxNormClient(baseURL string) *HTTPRxNormClient {
	return &HTTPRxNormClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchApproximate searches for drugs by approximate name match.
func (c *HTTPRxNormClient) SearchApproximate(ctx context.Context, name string) ([]RxNormCandidateRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-approximate-match?DRUG_NAME=%s", c.baseURL, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data struct {
			ApproximateGroup struct {
				Candidate []RxNormCandidateRaw `json:"candidate"`
			} `json:"approximateGroup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data.ApproximateGroup.Candidate, nil
}

// FetchSpellingSuggestions fetches spelling suggestions for a drug name.
func (c *HTTPRxNormClient) FetchSpellingSuggestions(ctx context.Context, name string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-spelling-suggestions?DRUG_NAME=%s", c.baseURL, url.QueryEscape(name))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data struct {
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

	return upstream.Data.SuggestionGroup.SuggestionList.Suggestion, nil
}

// FetchNDCs fetches NDC codes for the given RxCUI.
func (c *HTTPRxNormClient) FetchNDCs(ctx context.Context, rxcui string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-ndcs?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data struct {
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

	return upstream.Data.NDCGroup.NDCList.NDC, nil
}

// FetchGenericProduct fetches generic product concepts for the given RxCUI.
func (c *HTTPRxNormClient) FetchGenericProduct(ctx context.Context, rxcui string) ([]RxNormConceptRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-generic-product?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data struct {
			MinConceptGroup struct {
				MinConcept []RxNormConceptRaw `json:"minConcept"`
			} `json:"minConceptGroup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data.MinConceptGroup.MinConcept, nil
}

// FetchAllRelated fetches all related concept groups for the given RxCUI.
func (c *HTTPRxNormClient) FetchAllRelated(ctx context.Context, rxcui string) ([]RxNormConceptGroupRaw, error) {
	reqURL := fmt.Sprintf("%s/api/cache/rxnorm-all-related?RXCUI=%s", c.baseURL, url.QueryEscape(rxcui))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: upstream returned status %d", ErrUpstream, resp.StatusCode)
	}

	var upstream struct {
		Data struct {
			AllRelatedGroup struct {
				ConceptGroup []RxNormConceptGroupRaw `json:"conceptGroup"`
			} `json:"allRelatedGroup"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("%w: failed to decode response: %v", ErrUpstream, err)
	}

	return upstream.Data.AllRelatedGroup.ConceptGroup, nil
}
