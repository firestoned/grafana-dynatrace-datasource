import React, { ChangeEvent } from 'react';
import { InlineField, Input } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';

import { DataSource } from '../datasource';
import { DynatraceQuery, DynatraceDataSourceOptions } from '../types';

type Props = QueryEditorProps<DataSource, DynatraceQuery, DynatraceDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onMetricChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, metricSelector: e.target.value });
  };

  const onResolutionChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, resolution: e.target.value });
  };

  const onEntityChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, entitySelector: e.target.value });
  };

  const { metricSelector, resolution, entitySelector } = query;

  return (
    <div className="gf-form-group">
      <InlineField
        label="Metric selector"
        labelWidth={20}
        grow
        tooltip='A Dynatrace metric selector, e.g. builtin:host.cpu.usage:avg or builtin:service.response.time:splitBy("dt.entity.service"):avg'
      >
        <Input
          value={metricSelector ?? ''}
          placeholder="builtin:host.cpu.usage:avg"
          onChange={onMetricChange}
          onBlur={onRunQuery}
        />
      </InlineField>
      <InlineField
        label="Resolution"
        labelWidth={20}
        tooltip='Optional. e.g. "1m", "5m", "1h", or "Inf" for a single value. Empty = Dynatrace default.'
      >
        <Input
          width={20}
          value={resolution ?? ''}
          placeholder="1m"
          onChange={onResolutionChange}
          onBlur={onRunQuery}
        />
      </InlineField>
      <InlineField
        label="Entity selector"
        labelWidth={20}
        grow
        tooltip='Optional Dynatrace entity selector, e.g. type("HOST"),tag("env:prod")'
      >
        <Input
          value={entitySelector ?? ''}
          placeholder='type("HOST")'
          onChange={onEntityChange}
          onBlur={onRunQuery}
        />
      </InlineField>
    </div>
  );
}
