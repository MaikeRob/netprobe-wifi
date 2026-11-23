import React, { useState, useEffect, useCallback, useRef } from 'react';
import { TitleBar } from './components/TitleBar';
import { Header } from './components/Header';
import { Controls } from './components/Controls';
import { MetricsGrid } from './components/MetricsGrid';
import { HostTable } from './components/HostTable';
import { PortDrawer } from './components/PortDrawer';
import {
  DiscoveredHost,
  HostBatchEvent,
  SweepProgressEvent,
  SweepSummaryEvent,
  NetworkAdapter,
  PortResult,
  Capabilities,
} from './types/scanner';

import {
  GetNetworkInterfaces,
  GetDefaultInterface,
  CheckCapabilities,
  StartSweep,
  CancelSweep,
  ScanPorts,
  CancelPortScan,
} from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';

function formatErrorMessage(err: unknown): string {
  if (typeof err === 'string') return err;
  if (err instanceof Error) return err.message;
  return String(err);
}

export const App: React.FC = () => {
  // Network interfaces and target configuration
  const [adapters, setAdapters] = useState<NetworkAdapter[]>([]);
  const [selectedAdapter, setSelectedAdapter] = useState<NetworkAdapter | null>(null);
  const [cidr, setCidr] = useState('192.168.1.0/24');
  const [rateLimit, setRateLimit] = useState(100);
  const [timeoutMs, setTimeoutMs] = useState(1000);

  // Sweep state
  const [isSweeping, setIsSweeping] = useState(false);
  const [progress, setProgress] = useState<SweepProgressEvent | null>(null);
  const [hosts, setHosts] = useState<DiscoveredHost[]>([]);
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);

  // Port scan drawer state
  const [selectedHostForPort, setSelectedHostForPort] = useState<DiscoveredHost | null>(null);
  const [isScanningPorts, setIsScanningPorts] = useState(false);
  const [portResults, setPortResults] = useState<PortResult[]>([]);
  const [portScanError, setPortScanError] = useState<string | null>(null);
  const portScanSeqRef = useRef(0);

  const refreshSystem = useCallback(async () => {
    try {
      const caps = await CheckCapabilities();
      setCapabilities(caps);

      const ifaces = (await GetNetworkInterfaces()) || [];
      setAdapters(ifaces);

      if (ifaces.length === 0) {
        setSelectedAdapter(null);
        return;
      }

      try {
        const def = await GetDefaultInterface();
        setSelectedAdapter(def);
        if (def.cidr) {
          setCidr(def.cidr);
        }
      } catch {
        setSelectedAdapter(ifaces[0]);
        if (ifaces[0].cidr) {
          setCidr(ifaces[0].cidr);
        }
      }
    } catch (err) {
      setProgress({
        stage: 'error',
        totalHosts: 0,
        scannedHosts: 0,
        activeHosts: 0,
        icmpDiscovered: 0,
        arpDiscovered: 0,
        percent: 0,
        errorMessage: `Falha ao consultar interfaces de rede: ${formatErrorMessage(err)}`,
      });
    }
  }, []);

  // Load interfaces and capabilities on mount
  useEffect(() => {
    refreshSystem();

    // Listen to real-time events from Go backend
    const cancelProgress = EventsOn('sweep:progress', (p: SweepProgressEvent) => {
      setProgress(p);
      if (p.stage === 'finished' || p.stage === 'canceled' || p.stage === 'error') {
        setIsSweeping(false);
      }
    });

    const cancelBatch = EventsOn('sweep:batch', (b: HostBatchEvent) => {
      if (!b || !b.hosts) return;
      setHosts((prev) => {
        const map = new Map<string, DiscoveredHost>();
        for (const h of prev) {
          map.set(h.ip, h);
        }
        for (const h of b.hosts) {
          map.set(h.ip, h);
        }
        return Array.from(map.values()).sort((a, b) => a.ip.localeCompare(b.ip, undefined, { numeric: true }));
      });
    });

    const cancelSummary = EventsOn('sweep:summary', (_s: SweepSummaryEvent) => {
      setIsSweeping(false);
    });

    const cancelPortResult = EventsOn('port:result', (r: PortResult) => {
      setPortResults((prev) => {
        if (prev.some((item) => item.port === r.port)) {
          return prev;
        }
        return [...prev, r];
      });
    });

    return () => {
      cancelProgress();
      cancelBatch();
      cancelSummary();
      cancelPortResult();
      EventsOff('sweep:progress');
      EventsOff('sweep:batch');
      EventsOff('sweep:summary');
      EventsOff('port:result');
    };
  }, [refreshSystem]);

  const handleStartSweep = async () => {
    try {
      setHosts([]);
      setProgress(null);
      setIsSweeping(true);
      await StartSweep(cidr, selectedAdapter?.name || '', rateLimit, timeoutMs);
    } catch (err) {
      setIsSweeping(false);
      setProgress({
        stage: 'error',
        totalHosts: 0,
        scannedHosts: 0,
        activeHosts: 0,
        icmpDiscovered: 0,
        arpDiscovered: 0,
        percent: 0,
        errorMessage: formatErrorMessage(err),
      });
    }
  };

  const handleCancelSweep = async () => {
    try {
      await CancelSweep();
      setIsSweeping(false);
    } catch (err) {
      setProgress((prev) => ({
        stage: 'error',
        totalHosts: prev?.totalHosts || 0,
        scannedHosts: prev?.scannedHosts || 0,
        activeHosts: prev?.activeHosts || 0,
        icmpDiscovered: prev?.icmpDiscovered || 0,
        arpDiscovered: prev?.arpDiscovered || 0,
        percent: prev?.percent || 0,
        errorMessage: `Erro ao cancelar varredura: ${formatErrorMessage(err)}`,
      }));
    }
  };

  const handleSelectAdapter = (adapter: NetworkAdapter) => {
    setSelectedAdapter(adapter);
    if (adapter.cidr) {
      setCidr(adapter.cidr);
    }
  };

  const handleStartPortScan = async (targetIP: string, startPort: number, endPort: number, timeout: number) => {
    const seq = ++portScanSeqRef.current;
    try {
      setPortResults([]);
      setPortScanError(null);
      setIsScanningPorts(true);
      const results = await ScanPorts(targetIP, startPort, endPort, timeout);
      if (seq === portScanSeqRef.current && results) {
        setPortResults(results);
      }
    } catch (err) {
      if (seq === portScanSeqRef.current) {
        const msg = formatErrorMessage(err);
        if (!msg.includes('context canceled')) {
          setPortScanError(msg);
        }
      }
    } finally {
      if (seq === portScanSeqRef.current) {
        setIsScanningPorts(false);
      }
    }
  };

  const handleCancelPortScan = async () => {
    portScanSeqRef.current++;
    try {
      await CancelPortScan();
    } catch (err) {
      setPortScanError(`Erro ao cancelar escaneamento: ${formatErrorMessage(err)}`);
    } finally {
      setIsScanningPorts(false);
    }
  };

  const handleSelectHostForPort = (host: DiscoveredHost | null) => {
    if (isScanningPorts) {
      portScanSeqRef.current++;
      void CancelPortScan();
      setIsScanningPorts(false);
    }
    setSelectedHostForPort(host);
    setPortResults([]);
    setPortScanError(null);
  };

  return (
    <div className="h-screen w-screen flex flex-col bg-background text-stone-100 font-sans overflow-hidden">
      {/* Frameless TitleBar */}
      <TitleBar />

      {/* Main Workspace Body */}
      <main className="flex-1 flex flex-col p-4 gap-3.5 min-h-0 overflow-hidden">
        {/* Header with Title and Capabilities */}
        <Header
          capabilities={capabilities}
          isSweeping={isSweeping}
          onRefreshCapabilities={refreshSystem}
        />

        {/* Controls Panel */}
        <Controls
          adapters={adapters}
          selectedAdapter={selectedAdapter}
          onSelectAdapter={handleSelectAdapter}
          onRefreshAdapters={refreshSystem}
          cidr={cidr}
          onChangeCIDR={setCidr}
          rateLimit={rateLimit}
          onChangeRateLimit={setRateLimit}
          timeoutMs={timeoutMs}
          onChangeTimeoutMs={setTimeoutMs}
          isSweeping={isSweeping}
          onStartSweep={handleStartSweep}
          onCancelSweep={handleCancelSweep}
        />

        {/* Metrics & Progress Bar */}
        <MetricsGrid progress={progress} isSweeping={isSweeping} />

        {/* Live Discovered Hosts Table */}
        <HostTable
          hosts={hosts}
          onSelectHostForPortScan={(host) => handleSelectHostForPort(host)}
        />
      </main>

      {/* Option A: Slide-over Drawer for Port Scanning */}
      <PortDrawer
        isOpen={selectedHostForPort !== null}
        onClose={() => handleSelectHostForPort(null)}
        host={selectedHostForPort}
        onStartPortScan={handleStartPortScan}
        onCancelPortScan={handleCancelPortScan}
        isScanningPorts={isScanningPorts}
        portResults={portResults}
        errorMessage={portScanError}
      />
    </div>
  );
};
