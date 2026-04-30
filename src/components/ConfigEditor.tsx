import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';

import { DynatraceDataSourceOptions, DynatraceSecureJsonData } from '../types';

type Props = DataSourcePluginOptionsEditorProps<DynatraceDataSourceOptions, DynatraceSecureJsonData>;

export function ConfigEditor(props: Props) {
  const { options, onOptionsChange } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onUrlChange = (e: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, environmentUrl: e.target.value },
    });
  };

  const onTokenChange = (e: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...secureJsonData, apiToken: e.target.value },
    });
  };

  const onTokenReset = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...secureJsonFields, apiToken: false },
      secureJsonData: { ...secureJsonData, apiToken: '' },
    });
  };

  return (
    <>
      <InlineField
        label="Environment URL"
        labelWidth={20}
        tooltip="Your Dynatrace environment URL, e.g. https://abc12345.live.dynatrace.com (SaaS) or https://your-host/e/{env-id} (Managed)."
      >
        <Input
          width={50}
          value={jsonData.environmentUrl ?? ''}
          placeholder="https://abc12345.live.dynatrace.com"
          onChange={onUrlChange}
        />
      </InlineField>
      <InlineField
        label="API Token"
        labelWidth={20}
        tooltip="A Dynatrace API token with at least the metrics.read scope."
      >
        <SecretInput
          width={50}
          isConfigured={Boolean(secureJsonFields?.apiToken)}
          value={secureJsonData?.apiToken ?? ''}
          placeholder="dt0c01.XXXXXXXXXXXXXXXXXXXXXXXX..."
          onChange={onTokenChange}
          onReset={onTokenReset}
        />
      </InlineField>
    </>
  );
}
