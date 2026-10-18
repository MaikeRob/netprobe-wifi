import React, { useState } from 'react';
import { X, Play, Square, Activity, Server } from 'lucide-react';
import { DiscoveredHost, PortResult } from '../types/scanner';
import { formatDuration } from '../lib/utils';

interface PortDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  host: DiscoveredHost | null;
  onStartPortScan: (targetIP: string, startPort: number, endPort: number, timeoutMs: number) => void;
  onCancelPortScan: () => void;
  isScanningPorts: boolean;
  portResults: PortResult[];
}

export const PortDrawer: React.FC<PortDrawerProps> = ({
  isOpen,
  onClose,
  host,
  onStartPortScan,
  onCancelPortScan,
  isScanningPorts,
  portResults,
}) => {
  const [startPort, setStartPort] = useState(1);
  const [endPort, setEndPort] = useState(1024);
  const [timeoutMs, setTimeoutMs] = useState(1000);

  if (!isOpen || !host) return null;

  const openCount = portResults.filter((p) => p.state === 'open').length;
  const filteredCount = portResults.filter((p) => p.state === 'filtered').length;
  const closedCount = portResults.filter((p) => p.state === 'closed').length;

  const handleStart = () => {
    onStartPortScan(host.ip, startPort, endPort, timeoutMs);
  };

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm transition-opacity">
      {/* Click outside backdrop */}
      <div className="flex-1" onClick={onClose} />

      {/* Slide-over Drawer Panel */}
      <aside className="w-full max-w-md bg-surface border-l border-border-subtle shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-200">
        {/* Drawer Header */}
        <div className="p-4 border-b border-border-subtle flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2">
            <Server className="w-4 h-4 text-stone-400" />
            <h2 className="text-sm font-semibold text-stone-100">Varredura de Portas TCP</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-stone-400 hover:text-stone-200 hover:bg-stone-800 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Host Details Card */}
        <div className="p-4 border-b border-border-subtle bg-stone-900/40 space-y-2 shrink-0">
          <div className="flex items-baseline justify-between">
            <span className="font-mono text-base font-semibold text-stone-100">{host.ip}</span>
            <span className="text-xs text-stone-400 font-mono">{host.mac || 'Sem MAC'}</span>
          </div>
          <div className="text-xs text-stone-400 truncate">
            {host.vendor || 'Fabricante não identificado'}
          </div>
        </div>

        {/* Port Range Config */}
        <div className="p-4 border-b border-border-subtle space-y-3 shrink-0">
          <div className="grid grid-cols-3 gap-2.5">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-stone-200">Porta Inicial</label>
              <input
                type="number"
                min="1"
                max="65535"
                value={startPort}
                disabled={isScanningPorts}
                onChange={(e) => setStartPort(Math.max(1, parseInt(e.target.value, 10) || 1))}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-stone-200">Porta Final</label>
              <input
                type="number"
                min="1"
                max="65535"
                value={endPort}
                disabled={isScanningPorts}
                onChange={(e) => setEndPort(Math.min(65535, parseInt(e.target.value, 10) || 1024))}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-stone-200">Timeout (ms)</label>
              <input
                type="number"
                min="200"
                max="5000"
                step="100"
                value={timeoutMs}
                disabled={isScanningPorts}
                onChange={(e) => setTimeoutMs(Math.max(200, Math.min(5000, parseInt(e.target.value, 10) || 1000)))}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
          </div>

          {/* Quick presets */}
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => {
                setStartPort(1);
                setEndPort(1024);
              }}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              1–1024 (Padrão)
            </button>
            <button
              type="button"
              onClick={() => {
                setStartPort(20);
                setEndPort(445);
              }}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              Serviços Comuns
            </button>
            <button
              type="button"
              onClick={() => {
                setStartPort(1);
                setEndPort(8080);
              }}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              Até 8080
            </button>
          </div>

          {/* Action button */}
          {!isScanningPorts ? (
            <button
              onClick={handleStart}
              className="w-full bg-sage hover:bg-sage-hover text-stone-950 font-semibold text-xs sm:text-sm py-2 rounded-lg flex items-center justify-center gap-1.5 transition-all shadow-sm"
            >
              <Play className="w-4 h-4 fill-current" />
              <span>Escanear Portas</span>
            </button>
          ) : (
            <button
              onClick={onCancelPortScan}
              className="w-full bg-stone-800 hover:bg-red-950/40 hover:text-red-200 border border-red-800/60 text-stone-100 font-semibold text-xs sm:text-sm py-2 rounded-lg flex items-center justify-center gap-1.5 transition-all"
            >
              <Square className="w-4 h-4 fill-current" />
              <span>Cancelar Escaneamento</span>
            </button>
          )}
        </div>

        {/* Counter Summary Pills */}
        <div className="px-4 py-2.5 bg-stone-900/60 border-b border-border-subtle flex items-center justify-between text-xs shrink-0">
          <span className="text-stone-200 font-semibold">Resultados</span>
          <div className="flex items-center gap-2 font-mono text-xs">
            <span className="text-sage font-bold">{openCount} abertas</span>
            <span className="text-stone-500">•</span>
            <span className="text-amber-400 font-bold">{filteredCount} filtradas</span>
            <span className="text-stone-500">•</span>
            <span className="text-stone-300 font-bold">{closedCount} fechadas</span>
          </div>
        </div>

        {/* Results List */}
        <div className="flex-1 overflow-auto p-3">
          {portResults.length === 0 ? (
            <div className="h-48 flex flex-col items-center justify-center text-stone-300 text-sm gap-1.5">
              {isScanningPorts ? (
                <>
                  <Activity className="w-5 h-5 animate-spin text-sage mb-1" />
                  <span className="font-medium text-stone-200">Realizando handshake TCP...</span>
                </>
              ) : (
                <span className="font-medium text-stone-300">Clique em Escanear Portas para iniciar.</span>
              )}
            </div>
          ) : (
            <div className="space-y-1.5">
              {portResults.map((p) => {
                const isOpen = p.state === 'open';
                const isFiltered = p.state === 'filtered';

                return (
                  <div
                    key={p.port}
                    className="flex items-center justify-between p-2.5 rounded-lg bg-stone-900 border border-stone-800 text-xs sm:text-sm"
                  >
                    <div className="flex items-center gap-2.5">
                      <span className="font-mono font-bold text-stone-100 w-14">
                        {p.port}
                      </span>
                      {p.service ? (
                        <span className="text-stone-100 font-semibold">{p.service}</span>
                      ) : (
                        <span className="text-stone-400 italic">Desconhecido</span>
                      )}
                    </div>

                    <div className="flex items-center gap-2">
                      <span className="text-xs font-mono text-stone-300">
                        {formatDuration(p.latency)}
                      </span>
                      {isOpen ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-sage-subtle text-sage border border-sage/30">
                          Aberta
                        </span>
                      ) : isFiltered ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold bg-amber-950/40 text-amber-300 border border-amber-800/60">
                          Filtrada
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold text-stone-400 bg-stone-800">
                          Fechada
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </aside>
    </div>
  );
};
