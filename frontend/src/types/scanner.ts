export type SweepStage = 'starting' | 'icmp' | 'arp' | 'finished' | 'canceled' | 'error';

export interface DiscoveredHost {
  ip: string;
  mac: string;
  vendor: string;
  kind: 'vendor' | 'private' | 'unknown' | string;
  rtt: number; // nanoseconds
  discoveredBy: 'Ativo via ICMP' | 'Descoberto via ARP' | string;
  responded: boolean;
}

export interface HostBatchEvent {
  hosts: DiscoveredHost[];
}

export interface SweepProgressEvent {
  stage: SweepStage;
  totalHosts: number;
  scannedHosts: number;
  activeHosts: number;
  icmpDiscovered: number;
  arpDiscovered: number;
  percent: number;
  errorMessage?: string;
}

export interface SweepSummaryEvent {
  totalHosts: number;
  activeHosts: number;
  icmpDiscovered: number;
  arpDiscovered: number;
  duration: number; // nanoseconds
  reason?: string;
}

export interface NetworkAdapter {
  name: string;
  ip: string;
  cidr: string;
  mac: string;
  index: number;
}

export interface PortResult {
  port: number;
  state: 'open' | 'closed' | 'filtered' | string;
  service: string;
  latency: number; // nanoseconds
}

export interface Capabilities {
  CanSendICMP: boolean;
  Reason: string;
}
