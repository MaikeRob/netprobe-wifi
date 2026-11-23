import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PortDrawer } from './PortDrawer';
import { Controls } from './Controls';
import { DiscoveredHost, PortResult } from '../types/scanner';

const sampleHost: DiscoveredHost = {
  ip: '192.168.1.1',
  mac: '00:1A:2B:3C:4D:5E',
  vendor: 'Cisco Systems',
  kind: 'vendor',
  rtt: 2_000_000,
  discoveredBy: 'Ativo via ICMP',
  responded: true,
};

const samplePortResults: PortResult[] = [
  { port: 22, state: 'closed', service: 'SSH', latency: 1_000_000 },
  { port: 80, state: 'open', service: 'HTTP', latency: 1_500_000 },
  { port: 443, state: 'open', service: 'HTTPS', latency: 1_800_000 },
  { port: 8080, state: 'filtered', service: 'HTTP-Proxy', latency: 500_000_000 },
];

describe('PortDrawer', () => {
  it('defaults to showing only open ports and allows switching to all ports', () => {
    render(
      <PortDrawer
        isOpen={true}
        onClose={() => {}}
        host={sampleHost}
        onStartPortScan={() => {}}
        onCancelPortScan={() => {}}
        isScanningPorts={false}
        portResults={samplePortResults}
      />
    );

    // By default, "Abertas (2)" tab is active -> only ports 80 and 443 are displayed
    expect(screen.getByText('Abertas (2)')).toBeInTheDocument();
    expect(screen.getByText('80')).toBeInTheDocument();
    expect(screen.getByText('443')).toBeInTheDocument();
    expect(screen.queryByText('22')).not.toBeInTheDocument();

    // Switch to "Todas (4)" tab
    fireEvent.click(screen.getByText('Todas (4)'));
    expect(screen.getByText('22')).toBeInTheDocument();
    expect(screen.getByText('8080')).toBeInTheDocument();
  });

  it('allows free keyboard typing in port and timeout inputs and validates invalid ranges', () => {
    const onStart = vi.fn();
    render(
      <PortDrawer
        isOpen={true}
        onClose={() => {}}
        host={sampleHost}
        onStartPortScan={onStart}
        onCancelPortScan={() => {}}
        isScanningPorts={false}
        portResults={[]}
      />
    );

    const startInput = screen.getByLabelText('Porta Inicial');
    const endInput = screen.getByLabelText('Porta Final');
    const timeoutInput = screen.getByLabelText('Timeout (ms)');

    // User can clear and type "500" into timeout without immediate clamping to 200
    fireEvent.change(timeoutInput, { target: { value: '' } });
    fireEvent.change(timeoutInput, { target: { value: '5' } });
    expect((timeoutInput as HTMLInputElement).value).toBe('5');
    fireEvent.change(timeoutInput, { target: { value: '500' } });
    expect((timeoutInput as HTMLInputElement).value).toBe('500');

    // Invalid range: startPort > endPort shows clear error and prevents scan
    fireEvent.change(startInput, { target: { value: '1000' } });
    fireEvent.change(endInput, { target: { value: '80' } });
    fireEvent.click(screen.getByText('Escanear Portas'));

    expect(screen.getByText('A porta inicial não pode ser maior que a porta final.')).toBeInTheDocument();
    expect(onStart).not.toHaveBeenCalled();

    // Valid range triggers callback with parsed numeric values
    fireEvent.change(startInput, { target: { value: '20' } });
    fireEvent.change(endInput, { target: { value: '100' } });
    fireEvent.click(screen.getByText('Escanear Portas'));

    expect(onStart).toHaveBeenCalledWith('192.168.1.1', 20, 100, 500);
  });

  it('renders backend error message when provided', () => {
    render(
      <PortDrawer
        isOpen={true}
        onClose={() => {}}
        host={sampleHost}
        onStartPortScan={() => {}}
        onCancelPortScan={() => {}}
        isScanningPorts={false}
        portResults={[]}
        errorMessage="Falha de rede durante o scan de portas"
      />
    );

    expect(screen.getByText('Falha de rede durante o scan de portas')).toBeInTheDocument();
  });
});

describe('Controls (Network Disconnected Guard)', () => {
  it('shows warning banner and disables start button when no network adapters are active', () => {
    const onRefresh = vi.fn();
    render(
      <Controls
        adapters={[]}
        selectedAdapter={null}
        onSelectAdapter={() => {}}
        onRefreshAdapters={onRefresh}
        cidr="192.168.1.0/24"
        onChangeCIDR={() => {}}
        rateLimit={100}
        onChangeRateLimit={() => {}}
        timeoutMs={1000}
        onChangeTimeoutMs={() => {}}
        isSweeping={false}
        onStartSweep={() => {}}
        onCancelSweep={() => {}}
      />
    );

    expect(screen.getByText(/Sem conexão de rede ativa:/i)).toBeInTheDocument();
    const startBtn = screen.getByRole('button', { name: /Iniciar/i });
    expect(startBtn).toBeDisabled();
  });
});

