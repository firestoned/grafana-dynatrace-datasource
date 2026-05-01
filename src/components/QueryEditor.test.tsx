import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';

import { QueryEditor } from './QueryEditor';
import { DynatraceQuery } from '../types';

function setup(query: Partial<DynatraceQuery> = {}) {
  const onChange = jest.fn();
  const onRunQuery = jest.fn();
  const fullQuery: DynatraceQuery = {
    refId: 'A',
    metricSelector: '',
    resolution: '',
    entitySelector: '',
    ...query,
  };
  render(
    <QueryEditor
      query={fullQuery}
      onChange={onChange}
      onRunQuery={onRunQuery}
      datasource={{} as any}
    />
  );
  return { onChange, onRunQuery };
}

describe('QueryEditor', () => {
  it('renders all three inputs', () => {
    setup();
    expect(screen.getByPlaceholderText(/builtin:host/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText('1m')).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/HOST/)).toBeInTheDocument();
  });

  it('shows existing query values', () => {
    setup({
      metricSelector: 'builtin:host.cpu.usage:avg',
      resolution: '5m',
      entitySelector: 'type("HOST")',
    });
    expect(screen.getByDisplayValue('builtin:host.cpu.usage:avg')).toBeInTheDocument();
    expect(screen.getByDisplayValue('5m')).toBeInTheDocument();
    expect(screen.getByDisplayValue('type("HOST")')).toBeInTheDocument();
  });

  it('handles undefined query fields without crashing', () => {
    expect(() => setup({})).not.toThrow();
    expect((screen.getByPlaceholderText(/builtin:host/) as HTMLInputElement).value).toBe('');
  });

  it('updates metricSelector via onChange', () => {
    const { onChange } = setup();
    fireEvent.change(screen.getByPlaceholderText(/builtin:host/), {
      target: { value: 'builtin:host.cpu.usage:avg' },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ metricSelector: 'builtin:host.cpu.usage:avg' })
    );
  });

  it('updates resolution via onChange', () => {
    const { onChange } = setup();
    fireEvent.change(screen.getByPlaceholderText('1m'), { target: { value: '5m' } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ resolution: '5m' }));
  });

  it('updates entitySelector via onChange', () => {
    const { onChange } = setup();
    fireEvent.change(screen.getByPlaceholderText(/HOST/), {
      target: { value: 'tag("env:prod")' },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ entitySelector: 'tag("env:prod")' })
    );
  });

  it('triggers onRunQuery on metric blur', () => {
    const { onRunQuery } = setup();
    fireEvent.blur(screen.getByPlaceholderText(/builtin:host/));
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('triggers onRunQuery on resolution blur', () => {
    const { onRunQuery } = setup();
    fireEvent.blur(screen.getByPlaceholderText('1m'));
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('triggers onRunQuery on entity blur', () => {
    const { onRunQuery } = setup();
    fireEvent.blur(screen.getByPlaceholderText(/HOST/));
    expect(onRunQuery).toHaveBeenCalled();
  });

  it('preserves other query fields when one changes', () => {
    const { onChange } = setup({
      metricSelector: 'm',
      resolution: '5m',
      entitySelector: 'e',
    });
    fireEvent.change(screen.getByPlaceholderText('1m'), { target: { value: '1h' } });
    expect(onChange).toHaveBeenCalledWith({
      refId: 'A',
      metricSelector: 'm',
      resolution: '1h',
      entitySelector: 'e',
    });
  });
});
