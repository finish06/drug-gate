package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSPLClient_FetchSPLsByName_Success(t *testing.T) {
	entries := []SPLEntryRaw{
		{Title: "LIPITOR TABLET [PFIZER]", SetID: "abc-123", PublishedDate: "May 02, 2024", SPLVersion: 42},
		{Title: "LIPITOR TABLET [VIATRIS]", SetID: "def-456", PublishedDate: "Jun 27, 2024", SPLVersion: 7},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/spls-by-name" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("DRUGNAME") != "lipitor" {
			t.Errorf("unexpected DRUGNAME: %s", r.URL.Query().Get("DRUGNAME"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": entries})
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	result, err := c.FetchSPLsByName(context.Background(), "lipitor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].SetID != "abc-123" {
		t.Errorf("entry[0].SetID = %q, want %q", result[0].SetID, "abc-123")
	}
}

func TestHTTPSPLClient_FetchSPLsByName_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []SPLEntryRaw{}})
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	result, err := c.FetchSPLsByName(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestHTTPSPLClient_FetchSPLsByName_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	_, err := c.FetchSPLsByName(context.Background(), "lipitor")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestHTTPSPLClient_FetchSPLDetail_Success(t *testing.T) {
	entry := SPLEntryRaw{Title: "LIPITOR TABLET [PFIZER]", SetID: "abc-123", PublishedDate: "May 02, 2024", SPLVersion: 42}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/spl-detail" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("SETID") != "abc-123" {
			t.Errorf("unexpected SETID: %s", r.URL.Query().Get("SETID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []SPLEntryRaw{entry}})
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	result, err := c.FetchSPLDetail(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Title != "LIPITOR TABLET [PFIZER]" {
		t.Errorf("Title = %q, want %q", result.Title, "LIPITOR TABLET [PFIZER]")
	}
}

func TestHTTPSPLClient_FetchSPLDetail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	result, err := c.FetchSPLDetail(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for 404, got %+v", result)
	}
}

func TestHTTPSPLClient_FetchSPLXML_Success(t *testing.T) {
	xmlContent := `<?xml version="1.0"?><document><section><title>7 DRUG INTERACTIONS</title><text>Test interaction.</text></section></document>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/cache/spl-xml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(xmlContent))
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	data, err := c.FetchSPLXML(context.Background(), "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != xmlContent {
		t.Errorf("XML content mismatch")
	}
}

func TestHTTPSPLClient_FetchSPLXML_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	data, err := c.FetchSPLXML(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for 404, got %d bytes", len(data))
	}
}

func TestHTTPSPLClient_FetchSPLDetail_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	_, err := c.FetchSPLDetail(context.Background(), "some-setid")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestHTTPSPLClient_FetchSPLsByName_Unreachable(t *testing.T) {
	c := NewHTTPSPLClient("http://localhost:1")
	_, err := c.FetchSPLsByName(context.Background(), "warfarin")
	if err == nil {
		t.Error("expected error for unreachable, got nil")
	}
}

func TestHTTPSPLClient_FetchSPLXML_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewHTTPSPLClient(srv.URL)
	_, err := c.FetchSPLXML(context.Background(), "abc-123")
	if err == nil {
		t.Fatal("expected error for 502 response")
	}
}
