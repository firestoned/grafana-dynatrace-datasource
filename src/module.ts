import { DataSourcePlugin } from '@grafana/data';

import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { DynatraceQuery, DynatraceDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, DynatraceQuery, DynatraceDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
