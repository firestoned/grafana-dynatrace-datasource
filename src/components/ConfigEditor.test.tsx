import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';

import { ConfigEditor } from './ConfigEditor';

function makeOptions(overrides: any = {}): any {
  return {
    id: 1,
    uid: 'test',
    orgId: 1,
    name: 'test-ds',
    type: 'firestoned-dynatrace-datasource',
    typeName: 'Dynatrace',
    typeLogoUrl: '',
    access: 'proxy',
    url: '',
    user: '',
    database: '',
    basicAuth: false,
    basicAuthUser: '',
    withCredentials: false,
    isDefault: false,
    version: 1,
    readOnly: false,
    jsonData: { environmentUrl: '' },
    secureJsonFields: {},
    secureJsonData: {},
    ...overrides,
  };
}

function setup(overrides: any = {}) {
  const onOptionsChange = jest.fn();
  render(<ConfigEditor options={makeOptions(overrides)} onOptionsChange={onOptionsChange} />);
  return { onOptionsChange };
}

describe('ConfigEditor', () => {
  it('renders both inputs', () => {
    setup();
    expect(screen.getByPlaceholderText(/dynatrace\.com/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/dt0c01/)).toBeInTheDocument();
  });

  it('shows existing environmentUrl', () => {
    setup({ jsonData: { environmentUrl: 'https://abc.live.dynatrace.com' } });
    expect(screen.getByDisplayValue('https://abc.live.dynatrace.com')).toBeInTheDocument();
  });

  it('calls onOptionsChange when URL changes', () => {
    const { onOptionsChange } = setup();
    fireEvent.change(screen.getByPlaceholderText(/dynatrace\.com/), {
      target: { value: 'https://new.live.dynatrace.com' },
    });
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: { environmentUrl: 'https://new.live.dynatrace.com' },
      })
    );
  });

  it('preserves existing jsonData fields when URL changes', () => {
    const { onOptionsChange } = setup({
      jsonData: { environmentUrl: 'old', other: 'kept' } as any,
    });
    fireEvent.change(screen.getByPlaceholderText(/dynatrace\.com/), {
      target: { value: 'new' },
    });
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        jsonData: expect.objectContaining({ other: 'kept', environmentUrl: 'new' }),
      })
    );
  });

  it('calls onOptionsChange when token changes', () => {
    const { onOptionsChange } = setup();
    fireEvent.change(screen.getByPlaceholderText(/dt0c01/), {
      target: { value: 'dt0c01.SECRET' },
    });
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        secureJsonData: { apiToken: 'dt0c01.SECRET' },
      })
    );
  });

  it('shows reset affordance when token is configured', () => {
    setup({ secureJsonFields: { apiToken: true } });
    expect(screen.getByText(/reset/i)).toBeInTheDocument();
  });

  it('reset clears apiToken and the configured flag', () => {
    const { onOptionsChange } = setup({ secureJsonFields: { apiToken: true } });
    fireEvent.click(screen.getByText(/reset/i));
    expect(onOptionsChange).toHaveBeenCalledWith(
      expect.objectContaining({
        secureJsonFields: { apiToken: false },
        secureJsonData: { apiToken: '' },
      })
    );
  });

  it('renders blank inputs when jsonData is missing fields', () => {
    setup({ jsonData: {} });
    expect((screen.getByPlaceholderText(/dynatrace\.com/) as HTMLInputElement).value).toBe('');
  });
});
