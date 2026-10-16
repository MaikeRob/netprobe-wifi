import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MetricsGrid } from './MetricsGrid';
import { SweepProgressEvent } from '../types/scanner';

describe('MetricsGrid', () => {
  it('renders correctly with null progress', () => {
    render(<MetricsGrid progress={null} isSweeping={false} />);
    expect(screen.getByText('Total na Faixa')).toBeInTheDocument();
    expect(screen.getByText('Dispositivos Ativos')).toBeInTheDocument();
    expect(screen.getByText('Ativos via ICMP')).toBeInTheDocument();
    expect(screen.getByText('Via ARP (Firewall)')).toBeInTheDocument();
    expect(screen.getByText('Silenciosos / Offline')).toBeInTheDocument();
  });

  it('renders live progress and active counts', () => {
    const progress: SweepProgressEvent = {
      stage: 'icmp',
      totalHosts: 254,
      scannedHosts: 100,
      activeHosts: 5,
      icmpDiscovered: 5,
      arpDiscovered: 0,
      percent: 40,
    };

    render(<MetricsGrid progress={progress} isSweeping={true} />);
    expect(screen.getByText('Estágio 1: Varredura ICMP (Ping Sweep)')).toBeInTheDocument();
    expect(screen.getByText('40%')).toBeInTheDocument();
    expect(screen.getByText('254')).toBeInTheDocument();
    expect(screen.getByText('249')).toBeInTheDocument(); // 254 - 5 = 249 offline
  });

  it('renders ARP stage and finished state', () => {
    const progress: SweepProgressEvent = {
      stage: 'finished',
      totalHosts: 10,
      scannedHosts: 10,
      activeHosts: 4,
      icmpDiscovered: 2,
      arpDiscovered: 2,
      percent: 100,
    };

    render(<MetricsGrid progress={progress} isSweeping={false} />);
    expect(screen.getByText('Varredura Concluída')).toBeInTheDocument();
    expect(screen.getByText('100%')).toBeInTheDocument();
  });
});
