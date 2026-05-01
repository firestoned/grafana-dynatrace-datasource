// DataSourceWithBackend lives in @grafana/runtime and tries to wire up a
// backend transport at construction time. For unit testing we just want a
// stub base class that captures instanceSettings.
jest.mock('@grafana/runtime', () => ({
  DataSourceWithBackend: class {
    instanceSettings: unknown;
    constructor(instanceSettings: unknown) {
      this.instanceSettings = instanceSettings;
    }
  },
}));

import { CoreApp, DataSourceInstanceSettings } from '@grafana/data';

import { DataSource } from './datasource';
import { DEFAULT_QUERY, DynatraceDataSourceOptions } from './types';

const baseSettings = {
  id: 1,
  uid: 'test',
  type: 'firestoned-dynatrace-datasource',
  name: 'test-ds',
  meta: {},
  readOnly: false,
  access: 'proxy',
  jsonData: { environmentUrl: 'https://example.live.dynatrace.com' },
} as unknown as DataSourceInstanceSettings<DynatraceDataSourceOptions>;

describe('DataSource', () => {
  it('constructs without error', () => {
    expect(() => new DataSource(baseSettings)).not.toThrow();
  });

  it('getDefaultQuery returns DEFAULT_QUERY', () => {
    const ds = new DataSource(baseSettings);
    expect(ds.getDefaultQuery(CoreApp.PanelEditor)).toEqual(DEFAULT_QUERY);
  });

  it('getDefaultQuery is independent of CoreApp value', () => {
    const ds = new DataSource(baseSettings);
    expect(ds.getDefaultQuery(CoreApp.Dashboard)).toEqual(DEFAULT_QUERY);
    expect(ds.getDefaultQuery(CoreApp.Explore)).toEqual(DEFAULT_QUERY);
    expect(ds.getDefaultQuery(CoreApp.PanelEditor)).toEqual(DEFAULT_QUERY);
  });
});

describe('DEFAULT_QUERY', () => {
  it('has the expected shape', () => {
    expect(DEFAULT_QUERY).toEqual({
      metricSelector: '',
      resolution: '',
      entitySelector: '',
    });
  });
});
