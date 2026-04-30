package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Compile-time interface checks.
var (
	_ backend.QueryDataHandler   = (*Datasource)(nil)
	_ backend.CheckHealthHandler = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is one configured Dynatrace data source instance.
type Datasource struct {
	environmentURL string
	apiToken       string
	httpClient     *http.Client
}

// settings is the shape of the non-secret JSON sent by the frontend
// ConfigEditor. Field names must match `src/types.ts:DynatraceDataSourceOptions`.
type settings struct {
	EnvironmentURL string `json:"environmentUrl"`
}

// NewDatasource is the factory called by the SDK on instance create / update.
func NewDatasource(_ context.Context, s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var cfg settings
	if err := json.Unmarshal(s.JSONData, &cfg); err != nil {
		return nil, fmt.Errorf("invalid jsonData: %w", err)
	}
	if cfg.EnvironmentURL == "" {
		return nil, errors.New("environmentUrl is required")
	}

	token := s.DecryptedSecureJSONData["apiToken"]

	return &Datasource{
		environmentURL: strings.TrimRight(cfg.EnvironmentURL, "/"),
		apiToken:       token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Dispose is called by the SDK when an instance is replaced; nothing to do
// here since http.Client has no persistent resources to release.
func (d *Datasource) Dispose() {}

// queryModel matches the frontend DynatraceQuery type.
type queryModel struct {
	MetricSelector string `json:"metricSelector"`
	Resolution     string `json:"resolution,omitempty"`
	EntitySelector string `json:"entitySelector,omitempty"`
}

// QueryData runs each query in the request and returns one DataResponse per
// query, keyed by RefID.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	resp := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		resp.Responses[q.RefID] = d.handleQuery(ctx, q)
	}
	return resp, nil
}

func (d *Datasource) handleQuery(ctx context.Context, q backend.DataQuery) backend.DataResponse {
	var qm queryModel
	if err := json.Unmarshal(q.JSON, &qm); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "invalid query json: "+err.Error())
	}
	if qm.MetricSelector == "" {
		// Empty query — return an empty response rather than an error so
		// editing a panel doesn't constantly show error toasts.
		return backend.DataResponse{}
	}

	body, err := d.fetchMetrics(ctx, qm, q.TimeRange)
	if err != nil {
		log.DefaultLogger.Error("dynatrace request failed", "err", err)
		return backend.ErrDataResponse(backend.StatusBadGateway, err.Error())
	}

	frames, err := parseMetricsResponse(body, q.RefID)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusInternal, "parse error: "+err.Error())
	}
	return backend.DataResponse{Frames: frames}
}

// fetchMetrics calls GET /api/v2/metrics/query and returns the raw response body.
func (d *Datasource) fetchMetrics(ctx context.Context, qm queryModel, tr backend.TimeRange) ([]byte, error) {
	endpoint := d.environmentURL + "/api/v2/metrics/query"

	params := url.Values{}
	params.Set("metricSelector", qm.MetricSelector)
	params.Set("from", tr.From.Format(time.RFC3339))
	params.Set("to", tr.To.Format(time.RFC3339))
	if qm.Resolution != "" {
		params.Set("resolution", qm.Resolution)
	}
	if qm.EntitySelector != "" {
		params.Set("entitySelector", qm.EntitySelector)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Api-Token "+d.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dynatrace HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("dynatrace returned %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	return body, nil
}

// dtMetricsResponse is a partial mapping of the v2 metrics-query response.
// Only the fields used here are listed.
type dtMetricsResponse struct {
	Result []struct {
		MetricID string `json:"metricId"`
		Data     []struct {
			DimensionMap map[string]string `json:"dimensionMap"`
			Timestamps   []int64           `json:"timestamps"`
			Values       []*float64        `json:"values"` // null-able
		} `json:"data"`
	} `json:"result"`
}

func parseMetricsResponse(body []byte, refID string) (data.Frames, error) {
	var dt dtMetricsResponse
	if err := json.Unmarshal(body, &dt); err != nil {
		return nil, err
	}

	var frames data.Frames
	for _, r := range dt.Result {
		for _, series := range r.Data {
			times := make([]time.Time, len(series.Timestamps))
			for i, ms := range series.Timestamps {
				times[i] = time.UnixMilli(ms)
			}

			label := buildSeriesLabel(r.MetricID, series.DimensionMap)

			frame := data.NewFrame(label,
				data.NewField("time", nil, times),
				data.NewField(label, series.DimensionMap, series.Values),
			)
			frame.RefID = refID
			frames = append(frames, frame)
		}
	}
	return frames, nil
}

func buildSeriesLabel(metricID string, dims map[string]string) string {
	if len(dims) == 0 {
		return metricID
	}
	var sb strings.Builder
	sb.WriteString(metricID)
	sb.WriteString(" {")
	first := true
	for k, v := range dims {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(v)
	}
	sb.WriteString("}")
	return sb.String()
}

// CheckHealth implements the "Save & Test" button. It hits a cheap endpoint
// to verify URL + token before the user tries to run real queries.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.apiToken == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "API token is missing — set one in the data source config and Save & Test again.",
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.environmentURL+"/api/v2/metrics?pageSize=1", nil)
	if err != nil {
		return &backend.CheckHealthResult{Status: backend.HealthStatusError, Message: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Api-Token "+d.apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Could not reach Dynatrace: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Authentication failed (%d). Check the API token has the metrics.read scope.", resp.StatusCode),
		}, nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Dynatrace returned %d: %s", resp.StatusCode, truncate(string(body), 200)),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Connected to Dynatrace successfully.",
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
