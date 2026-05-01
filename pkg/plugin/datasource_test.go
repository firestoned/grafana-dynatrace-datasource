package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

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

const multiSeriesResponse = `{
  "result": [
    {
      "metricId": "builtin:host.cpu.usage:avg",
      "data": [
        {"dimensionMap": {"dt.entity.host": "HOST-1"}, "timestamps": [1714435200000], "values": [10.0]},
        {"dimensionMap": {"dt.entity.host": "HOST-2"}, "timestamps": [1714435200000], "values": [20.0]}
      ]
    },
    {
      "metricId": "builtin:host.mem.usage:avg",
      "data": [{"dimensionMap": {"dt.entity.host": "HOST-1"}, "timestamps": [1714435200000], "values": [50.0]}]
    }
  ]
}`

const nullValuesResponse = `{
  "result": [{
    "metricId": "m1",
    "data": [{
      "dimensionMap": {"k": "v"},
      "timestamps": [1, 2, 3],
      "values": [1.0, null, 3.0]
    }]
  }]
}`

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

func makeRequest(refID string, qm queryModel) *backend.QueryDataRequest {
	queryJSON, _ := json.Marshal(qm)
	return &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{
			RefID: refID,
			JSON:  queryJSON,
			TimeRange: backend.TimeRange{
				From: time.Now().Add(-time.Hour),
				To:   time.Now(),
			},
		}},
	}
}

// closedServerURL returns a URL pointing at an httptest server that's already
// been closed — connections to it fail at TCP level. Useful for network-error
// paths without needing to grab a free port and cross fingers.
func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

// ---------------------------------------------------------------------------
// NewDatasource
// ---------------------------------------------------------------------------

func TestNewDatasource_Success(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"environmentUrl":"https://example.live.dynatrace.com"}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "secret"},
	}
	inst, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ds := inst.(*Datasource)
	if ds.environmentURL != "https://example.live.dynatrace.com" {
		t.Errorf("environmentURL: got %q", ds.environmentURL)
	}
	if ds.apiToken != "secret" {
		t.Errorf("apiToken: got %q", ds.apiToken)
	}
	if ds.httpClient == nil {
		t.Error("httpClient was nil")
	}
}

func TestNewDatasource_TrimsTrailingSlash(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData:                []byte(`{"environmentUrl":"https://example.live.dynatrace.com/"}`),
		DecryptedSecureJSONData: map[string]string{"apiToken": "secret"},
	}
	inst, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := inst.(*Datasource).environmentURL; got != "https://example.live.dynatrace.com" {
		t.Errorf("trailing slash not trimmed: %q", got)
	}
}

func TestNewDatasource_TrimsMultipleTrailingSlashes(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"environmentUrl":"https://example.com///"}`),
	}
	inst, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got := inst.(*Datasource).environmentURL; got != "https://example.com" {
		t.Errorf("got %q", got)
	}
}

func TestNewDatasource_InvalidJSONData(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{not json`),
	}
	_, err := NewDatasource(context.Background(), settings)
	if err == nil {
		t.Fatal("expected error for invalid jsonData")
	}
	if !strings.Contains(err.Error(), "invalid jsonData") {
		t.Errorf("error message: %v", err)
	}
}

func TestNewDatasource_MissingEnvironmentURL(t *testing.T) {
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{}`),
	}
	_, err := NewDatasource(context.Background(), settings)
	if err == nil {
		t.Fatal("expected error for missing environmentUrl")
	}
	if !strings.Contains(err.Error(), "environmentUrl is required") {
		t.Errorf("error message: %v", err)
	}
}

func TestNewDatasource_NoToken(t *testing.T) {
	// Token absent at construction is permitted; CheckHealth surfaces it.
	settings := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"environmentUrl":"https://example.live.dynatrace.com"}`),
	}
	inst, err := NewDatasource(context.Background(), settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.(*Datasource).apiToken != "" {
		t.Error("expected empty apiToken")
	}
}

// ---------------------------------------------------------------------------
// QueryData / handleQuery
// ---------------------------------------------------------------------------

func TestQueryData_Success(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Api-Token test-token" {
			t.Errorf("auth header: got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("accept header: got %q", got)
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

	resp, err := ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "builtin:host.cpu.usage:avg"}))
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	r := resp.Responses["A"]
	if r.Error != nil {
		t.Fatalf("response error: %v", r.Error)
	}
	if len(r.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(r.Frames))
	}
	if r.Frames[0].Rows() != 2 {
		t.Errorf("expected 2 rows, got %d", r.Frames[0].Rows())
	}
	if r.Frames[0].Meta == nil || r.Frames[0].Meta.Type != data.FrameTypeTimeSeriesMulti {
		t.Errorf("expected FrameTypeTimeSeriesMulti meta, got %+v", r.Frames[0].Meta)
	}
	if r.Frames[0].RefID != "A" {
		t.Errorf("frame RefID: %q", r.Frames[0].RefID)
	}
}

func TestQueryData_EmptyMetricSelector(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for empty metricSelector")
	})
	resp, err := ds.QueryData(context.Background(), makeRequest("A", queryModel{}))
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	if resp.Responses["A"].Error != nil {
		t.Errorf("empty selector should be a no-op, got: %v", resp.Responses["A"].Error)
	}
	if len(resp.Responses["A"].Frames) != 0 {
		t.Errorf("expected zero frames, got %d", len(resp.Responses["A"].Frames))
	}
}

func TestQueryData_InvalidQueryJSON(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for invalid query JSON")
	})
	req := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{
			RefID:     "A",
			JSON:      []byte(`{not json`),
			TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
		}},
	}
	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	if resp.Responses["A"].Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(resp.Responses["A"].Error.Error(), "invalid query json") {
		t.Errorf("unexpected error: %v", resp.Responses["A"].Error)
	}
}

func TestQueryData_MultipleQueries(t *testing.T) {
	calls := 0
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleResponse))
	})
	queryAJSON, _ := json.Marshal(queryModel{MetricSelector: "m1"})
	queryBJSON, _ := json.Marshal(queryModel{MetricSelector: "m2"})
	req := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{RefID: "A", JSON: queryAJSON, TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()}},
			{RefID: "B", JSON: queryBJSON, TimeRange: backend.TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()}},
		},
	}
	resp, err := ds.QueryData(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryData error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 upstream calls, got %d", calls)
	}
	if _, ok := resp.Responses["A"]; !ok {
		t.Error("missing response A")
	}
	if _, ok := resp.Responses["B"]; !ok {
		t.Error("missing response B")
	}
}

func TestQueryData_ServerError(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	})
	resp, _ := ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "x"}))
	if resp.Responses["A"].Error == nil {
		t.Fatal("expected error from 500 response")
	}
	if !strings.Contains(resp.Responses["A"].Error.Error(), "500") {
		t.Errorf("expected 500 in error, got: %v", resp.Responses["A"].Error)
	}
}

func TestQueryData_4xxIncludesBodySnippet(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid metric selector syntax"}`))
	})
	resp, _ := ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "x"}))
	if resp.Responses["A"].Error == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(resp.Responses["A"].Error.Error(), "invalid metric selector syntax") {
		t.Errorf("expected response body in error, got: %v", resp.Responses["A"].Error)
	}
}

func TestQueryData_NetworkError(t *testing.T) {
	ds := &Datasource{
		environmentURL: closedServerURL(t),
		apiToken:       "tok",
		httpClient:     &http.Client{Timeout: 500 * time.Millisecond},
	}
	resp, _ := ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "x"}))
	if resp.Responses["A"].Error == nil {
		t.Error("expected network error")
	}
}

func TestQueryData_MalformedResponseBody(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	})
	resp, _ := ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "x"}))
	if resp.Responses["A"].Error == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(resp.Responses["A"].Error.Error(), "parse error") {
		t.Errorf("expected parse error message, got: %v", resp.Responses["A"].Error)
	}
}

func TestQueryData_ContextCancellation(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		// Block longer than the test will wait, simulating a slow upstream.
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	resp, _ := ds.QueryData(ctx, makeRequest("A", queryModel{MetricSelector: "x"}))
	if resp.Responses["A"].Error == nil {
		t.Error("expected context deadline error")
	}
}

// ---------------------------------------------------------------------------
// fetchMetrics — query parameter assembly
// ---------------------------------------------------------------------------

func TestFetchMetrics_AllOptionalParams(t *testing.T) {
	var captured *http.Request
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	_, _ = ds.QueryData(context.Background(), makeRequest("A", queryModel{
		MetricSelector: "m1",
		Resolution:     "5m",
		EntitySelector: `type("HOST")`,
	}))
	if captured == nil {
		t.Fatal("upstream not called")
	}
	q := captured.URL.Query()
	if q.Get("metricSelector") != "m1" {
		t.Errorf("metricSelector: %q", q.Get("metricSelector"))
	}
	if q.Get("resolution") != "5m" {
		t.Errorf("resolution: %q", q.Get("resolution"))
	}
	if q.Get("entitySelector") != `type("HOST")` {
		t.Errorf("entitySelector: %q", q.Get("entitySelector"))
	}
	if q.Get("from") == "" || q.Get("to") == "" {
		t.Error("from/to not set")
	}
}

func TestFetchMetrics_OmitsEmptyOptionalParams(t *testing.T) {
	var captured *http.Request
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	_, _ = ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "m1"}))
	q := captured.URL.Query()
	if q.Has("resolution") {
		t.Error("empty resolution should be omitted")
	}
	if q.Has("entitySelector") {
		t.Error("empty entitySelector should be omitted")
	}
}

func TestFetchMetrics_TimeRangeIsRFC3339(t *testing.T) {
	var captured *http.Request
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	_, _ = ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "m"}))
	q := captured.URL.Query()
	if _, err := time.Parse(time.RFC3339, q.Get("from")); err != nil {
		t.Errorf("from not RFC3339: %q (%v)", q.Get("from"), err)
	}
	if _, err := time.Parse(time.RFC3339, q.Get("to")); err != nil {
		t.Errorf("to not RFC3339: %q (%v)", q.Get("to"), err)
	}
}

// ---------------------------------------------------------------------------
// parseMetricsResponse
// ---------------------------------------------------------------------------

func TestParseMetricsResponse_Single(t *testing.T) {
	frames, err := parseMetricsResponse([]byte(sampleResponse), "A")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	if frames[0].RefID != "A" {
		t.Errorf("RefID: %q", frames[0].RefID)
	}
	if frames[0].Rows() != 2 {
		t.Errorf("expected 2 rows, got %d", frames[0].Rows())
	}
}

func TestParseMetricsResponse_MultipleSeries(t *testing.T) {
	frames, err := parseMetricsResponse([]byte(multiSeriesResponse), "Z")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 2 hosts under cpu + 1 host under mem = 3 frames
	if len(frames) != 3 {
		t.Fatalf("expected 3 frames, got %d", len(frames))
	}
	for _, f := range frames {
		if f.RefID != "Z" {
			t.Errorf("RefID: %q", f.RefID)
		}
		if f.Meta == nil || f.Meta.Type != data.FrameTypeTimeSeriesMulti {
			t.Errorf("frame meta missing or wrong: %+v", f.Meta)
		}
	}
}

func TestParseMetricsResponse_NullValues(t *testing.T) {
	frames, err := parseMetricsResponse([]byte(nullValuesResponse), "A")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame")
	}
	values := frames[0].Fields[1]
	if values.Len() != 3 {
		t.Errorf("expected 3 values, got %d", values.Len())
	}
	// Middle value is null → nil *float64
	v, ok := values.At(1).(*float64)
	if !ok {
		t.Fatalf("type assertion failed: %T", values.At(1))
	}
	if v != nil {
		t.Errorf("expected nil at index 1, got %v", *v)
	}
	// First value is non-nil
	v0, _ := values.At(0).(*float64)
	if v0 == nil || *v0 != 1.0 {
		t.Errorf("expected 1.0 at index 0")
	}
}

func TestParseMetricsResponse_EmptyResult(t *testing.T) {
	frames, err := parseMetricsResponse([]byte(`{"result":[]}`), "A")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("expected 0 frames, got %d", len(frames))
	}
}

func TestParseMetricsResponse_ResultWithoutData(t *testing.T) {
	body := `{"result":[{"metricId":"m1","data":[]}]}`
	frames, err := parseMetricsResponse([]byte(body), "A")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("expected 0 frames for empty data array, got %d", len(frames))
	}
}

func TestParseMetricsResponse_InvalidJSON(t *testing.T) {
	_, err := parseMetricsResponse([]byte(`{not json`), "A")
	if err == nil {
		t.Error("expected error")
	}
}

func TestParseMetricsResponse_TimestampsConvertedToMillis(t *testing.T) {
	body := `{"result":[{"metricId":"m","data":[{"dimensionMap":{},"timestamps":[1714435200000],"values":[1.0]}]}]}`
	frames, err := parseMetricsResponse([]byte(body), "A")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tField := frames[0].Fields[0]
	got, ok := tField.At(0).(time.Time)
	if !ok {
		t.Fatalf("expected time.Time at index 0, got %T", tField.At(0))
	}
	want := time.UnixMilli(1714435200000)
	if !got.Equal(want) {
		t.Errorf("timestamp: got %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// buildSeriesLabel — sorted, deterministic
// ---------------------------------------------------------------------------

func TestBuildSeriesLabel(t *testing.T) {
	cases := []struct {
		name string
		m    string
		dims map[string]string
		want string
	}{
		{"no dims (nil)", "metric.id", nil, "metric.id"},
		{"no dims (empty map)", "metric.id", map[string]string{}, "metric.id"},
		{"one dim", "m", map[string]string{"k": "v"}, "m {k=v}"},
		{"sorted multi", "metric", map[string]string{"z": "1", "a": "2", "m": "3"}, "metric {a=2, m=3, z=1}"},
		{"special chars in value", "m", map[string]string{"k": "with space"}, "m {k=with space}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildSeriesLabel(c.m, c.dims); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuildSeriesLabel_Deterministic(t *testing.T) {
	dims := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"}
	first := buildSeriesLabel("m", dims)
	for i := 0; i < 200; i++ {
		if got := buildSeriesLabel("m", dims); got != first {
			t.Fatalf("non-deterministic: %q != %q (iter %d)", got, first, i)
		}
	}
}

// ---------------------------------------------------------------------------
// CheckHealth
// ---------------------------------------------------------------------------

func TestCheckHealth_OK(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Api-Token test-token" {
			t.Errorf("auth header: %q", got)
		}
		if r.URL.Path != "/api/v2/metrics" {
			t.Errorf("path: %q", r.URL.Path)
		}
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
	if !strings.Contains(res.Message, "Connected") {
		t.Errorf("expected success message, got %q", res.Message)
	}
}

func TestCheckHealth_NoToken(t *testing.T) {
	ds := &Datasource{environmentURL: "https://example.com", apiToken: "", httpClient: http.DefaultClient}
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error for missing token, got %v", res.Status)
	}
	if !strings.Contains(res.Message, "API token is missing") {
		t.Errorf("expected 'API token is missing' message, got %q", res.Message)
	}
}

func TestCheckHealth_Unauthorized(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error, got %v", res.Status)
	}
	if !strings.Contains(res.Message, "Authentication failed") {
		t.Errorf("expected auth error, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "401") {
		t.Errorf("expected 401 in message, got %q", res.Message)
	}
}

func TestCheckHealth_Forbidden(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error for 403")
	}
	if !strings.Contains(res.Message, "403") {
		t.Errorf("expected 403 in message, got %q", res.Message)
	}
}

func TestCheckHealth_OtherClientError(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	})
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error for 400")
	}
	if !strings.Contains(res.Message, "400") {
		t.Errorf("expected 400 in message, got %q", res.Message)
	}
}

func TestCheckHealth_ServerError(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Errorf("expected Error for 500")
	}
}

func TestCheckHealth_NetworkError(t *testing.T) {
	ds := &Datasource{
		environmentURL: closedServerURL(t),
		apiToken:       "tok",
		httpClient:     &http.Client{Timeout: 500 * time.Millisecond},
	}
	res, _ := ds.CheckHealth(context.Background(), nil)
	if res.Status != backend.HealthStatusError {
		t.Error("expected Error on network failure")
	}
	if !strings.Contains(res.Message, "Could not reach Dynatrace") {
		t.Errorf("expected 'Could not reach' message, got %q", res.Message)
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"", 5, ""},
		{"abc", 5, "abc"},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "abcde..."},
		{"long-string-here", 4, "long..."},
		{"x", 0, "..."},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Dispose — must not panic, callable on zero/idle client
// ---------------------------------------------------------------------------

func TestDispose_NoPanic(t *testing.T) {
	ds := &Datasource{httpClient: &http.Client{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Dispose panicked: %v", r)
		}
	}()
	ds.Dispose()
}

func TestDispose_AfterUse(t *testing.T) {
	ds, _ := newTestDS(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	})
	_, _ = ds.QueryData(context.Background(), makeRequest("A", queryModel{MetricSelector: "x"}))
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Dispose panicked after use: %v", r)
		}
	}()
	ds.Dispose()
}
