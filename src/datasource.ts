import { DataSourceInstanceSettings, CoreApp } from '@grafana/data';
import { DataSourceWithBackend } from '@grafana/runtime';

import { DynatraceQuery, DynatraceDataSourceOptions, DEFAULT_QUERY } from './types';

/**
 * Frontend DataSource. Because the plugin has a backend (`backend: true` in
 * plugin.json), all queries and CheckHealth calls are proxied to the Go
 * binary; we don't make HTTP calls from the browser. This keeps the API
 * token server-side and avoids CORS issues against Dynatrace.
 */
export class DataSource extends DataSourceWithBackend<DynatraceQuery, DynatraceDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<DynatraceDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<DynatraceQuery> {
    return DEFAULT_QUERY;
  }
}
