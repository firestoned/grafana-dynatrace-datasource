package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const sampleResponse = `{
  "totalCount": 1,
  "nextPageKey": null,
  "resolution": "1m",
  "result": [{
    "metricId": "builtin:host.cpu.usage:avg",
    "data": [{
      "dimensions": ["HOST-1"],
      "dimensionMap": {"dt.entity.host": "HOST-1"},
      "timestamps": [1714435200000, 1714435260000],
      "values": [12.5, 15.0]
    }]
  }]
}`

// newTestDS spins up a fake Dynatrace server and returns a Datasource pointed
// at it. The handler is called for every request — assert in there.
func newTestDS(t *testing.T, handler http.HandlerFunc) (*Datasource, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &Datasource{
		environmentURL: srv.URL,
		apiToken:       "test-token",
		httpClient:     srv.Client(),
	}, srv
}

func TestQueryData_Success(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Api-Token test-token" {
			t.Errorf("auth header: got %q", got)
		}
		if r.URL.Path != "/api/v2/metrics/query" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("metricSelector"); got != "builtin:host.cpu.usage:avg" {
			t.Errorf("metricSelector: got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResponse))
	})

	queryJSON, _ := json.Marshal(queryModel{MetricSelector: "builtin:host.cpu.usage:avg"})
	req := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{
			RefID: "A",
			JSON:  queryJSON,
			TimeRange: backend.TimeRange{
				From: time.Now().Add(-time.Hour),
				To:   time.Now(),
			},
		}},
	}

	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	r, ok := resp.Responses["A"]
	if !ok {
		t.Fatal("missing response for refID A")
	}
	if r.Error != nil {
		t.Fatalf("response error: %v", r.Error)
	}
	if len(r.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(r.Frames))
	}
	if r.Frames[0].Rows() != 2 {
		t.Errorf("expected 2 rows, got %d", r.Frames[0].Rows())
	}
}

func TestQueryData_EmptyMetricSelector(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for empty metricSelector")
	})

	queryJSON, _ := json.Marshal(queryModel{MetricSelector: ""})
	req := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{
			RefID:     "A",
			JSON:      queryJSON,
			TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
		}},
	}
	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	if resp.Responses["A"].Error != nil {
		t.Errorf("empty selector should be a no-op, got: %v", resp.Responses["A"].Error)
	}
}

func TestCheckHealth_OK(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"metrics":[]}`))
	})
	res, err := ds.CheckHealth(context.Background(), nil)
	if err != nil {
		t.Fatalf("CheckHealth error: %v", err)
	}
	if res.Status != backend.HealthStatusOk {
		t.Errorf("expected Ok, got %v: %s", res.Status, res.Message)
	}
}

func TestCheckHealth_Unauthorized(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	res, err := ds.CheckHealth(context.Background(), nil)
	if err != nil {
		t.Fatalf("CheckHealth error: %v", err)
	}
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error, got %v", res.Status)
	}
}

func TestCheckHealth_NoToken(t *testing.T) {
	ds := &Datasource{environmentURL: "https://example.com", apiToken: "", httpClient: http.DefaultClient}
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error for missing token, got %v", res.Status)
	}
}
