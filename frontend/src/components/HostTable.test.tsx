import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { HostTable } from './HostTable';
import { DiscoveredHost } from '../types/scanner';

describe('HostTable', () => {
  const sampleHosts: DiscoveredHost[] = [
    {
      ip: '192.168.1.1',
      mac: '00:1A:2B:3C:4D:5E',
      vendor: 'Cisco Systems',
      kind: 'vendor',
      rtt: 2_500_000,
      discoveredBy: 'Ativo via ICMP',
      responded: true,
    },
    {
      ip: '192.168.1.50',
      mac: 'DA:A1:19:12:34:56',
      vendor: 'Private / Randomized MAC',
      kind: 'private',
      rtt: 0,
      discoveredBy: 'Descoberto via ARP',
      responded: false,
    },
  ];

  it('renders empty state when no hosts are present', () => {
    render(<HostTable hosts={[]} onSelectHostForPortScan={() => {}} />);
    expect(screen.getByText('Nenhum dispositivo encontrado na varredura.')).toBeInTheDocument();
  });

  it('renders host rows with correct badges and formatting', () => {
    render(<HostTable hosts={sampleHosts} onSelectHostForPortScan={() => {}} />);
    expect(screen.getByText('192.168.1.1')).toBeInTheDocument();
    expect(screen.getByText('00:1A:2B:3C:4D:5E')).toBeInTheDocument();
    expect(screen.getByText('Cisco Systems')).toBeInTheDocument();
    expect(screen.getByText('Ativo via ICMP')).toBeInTheDocument();

    expect(screen.getByText('192.168.1.50')).toBeInTheDocument();
    expect(screen.getByText('DA:A1:19:12:34:56')).toBeInTheDocument();
    expect(screen.getByText('MAC Randômico / Privado')).toBeInTheDocument();
    expect(screen.getByText('Descoberto via ARP')).toBeInTheDocument();
  });

  it('filters hosts by search query', () => {
    render(<HostTable hosts={sampleHosts} onSelectHostForPortScan={() => {}} />);
    const searchInput = screen.getByPlaceholderText('Filtrar por IP, MAC ou Fabricante...');
    
    // Search for Cisco
    fireEvent.change(searchInput, { target: { value: 'cisco' } });
    expect(screen.getByText('192.168.1.1')).toBeInTheDocument();
    expect(screen.queryByText('192.168.1.50')).not.toBeInTheDocument();

    // Search for ARP
    fireEvent.change(searchInput, { target: { value: 'ARP' } });
    expect(screen.queryByText('192.168.1.1')).not.toBeInTheDocument();
    expect(screen.getByText('192.168.1.50')).toBeInTheDocument();
  });

  it('invokes callback when clicking port scan button', () => {
    const handlePortScan = vi.fn();
    render(<HostTable hosts={sampleHosts} onSelectHostForPortScan={handlePortScan} />);
    const portButtons = screen.getAllByText('Portas');
    fireEvent.click(portButtons[0]);
    expect(handlePortScan).toHaveBeenCalledWith(sampleHosts[0]);
  });
});
