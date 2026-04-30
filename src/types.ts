import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

/**
 * A single Dynatrace metrics query.
 *
 * Maps roughly to the Dynatrace v2 metrics query API.
 * See: https://docs.dynatrace.com/docs/dynatrace-api/environment-api/metric-v2/get-data-points
 */
export interface DynatraceQuery extends DataQuery {
  /** A Dynatrace metric selector, e.g. "builtin:host.cpu.usage:avg". */
  metricSelector: string;
  /** Optional resolution: "1m", "5m", "1h", "Inf", etc. */
  resolution?: string;
  /** Optional entity selector, e.g. 'type("HOST")'. */
  entitySelector?: string;
}

export const DEFAULT_QUERY: Partial<DynatraceQuery> = {
  metricSelector: '',
  resolution: '',
  entitySelector: '',
};

/**
 * Non-secret data source configuration. Stored as JSON in Grafana's database.
 */
export interface DynatraceDataSourceOptions extends DataSourceJsonData {
  /** Dynatrace environment URL, e.g. https://abc12345.live.dynatrace.com */
  environmentUrl: string;
}

/**
 * Secret configuration. Encrypted at rest by Grafana, never returned to the
 * frontend after save (only a "configured" boolean is returned via
 * `secureJsonFields`).
 */
export interface DynatraceSecureJsonData {
  apiToken?: string;
}
