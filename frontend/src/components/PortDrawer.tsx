import React, { useState, useMemo, useEffect } from 'react';
import { X, Play, Square, Activity, Server, AlertCircle } from 'lucide-react';
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
  errorMessage?: string | null;
}

type PortFilterMode = 'open' | 'all';

export const PortDrawer: React.FC<PortDrawerProps> = ({
  isOpen,
  onClose,
  host,
  onStartPortScan,
  onCancelPortScan,
  isScanningPorts,
  portResults,
  errorMessage,
}) => {
  const [startPortInput, setStartPortInput] = useState('1');
  const [endPortInput, setEndPortInput] = useState('1024');
  const [timeoutInput, setTimeoutInput] = useState('1000');
  const [validationError, setValidationError] = useState<string | null>(null);
  const [filterMode, setFilterMode] = useState<PortFilterMode>('open');

  const openPorts = useMemo(() => portResults.filter((p) => p.state === 'open'), [portResults]);
  const filteredCount = useMemo(() => portResults.filter((p) => p.state === 'filtered').length, [portResults]);
  const closedCount = useMemo(() => portResults.filter((p) => p.state === 'closed').length, [portResults]);

  // Sort open ports to the top when viewing "all", and keep ascending port order within each state group
  const displayedPorts = useMemo(() => {
    if (filterMode === 'open') {
      return openPorts;
    }
    const rank = (state: string) => (state === 'open' ? 0 : state === 'filtered' ? 1 : 2);
    return [...portResults].sort((a, b) => {
      const diff = rank(a.state) - rank(b.state);
      return diff !== 0 ? diff : a.port - b.port;
    });
  }, [portResults, openPorts, filterMode]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen || !host) return null;

  const handleStart = () => {
    const startPort = parseInt(startPortInput, 10);
    const endPort = parseInt(endPortInput, 10);
    const timeoutMs = parseInt(timeoutInput, 10);

    if (isNaN(startPort) || isNaN(endPort) || startPort < 1 || endPort > 65535) {
      setValidationError('As portas inicial e final devem estar entre 1 e 65535.');
      return;
    }
    if (startPort > endPort) {
      setValidationError('A porta inicial não pode ser maior que a porta final.');
      return;
    }
    if (isNaN(timeoutMs) || timeoutMs < 200 || timeoutMs > 5000) {
      setValidationError('O timeout deve estar entre 200 ms e 5000 ms.');
      return;
    }

    setValidationError(null);
    onStartPortScan(host.ip, startPort, endPort, timeoutMs);
  };

  const applyPreset = (start: number, end: number) => {
    setStartPortInput(String(start));
    setEndPortInput(String(end));
    setValidationError(null);
  };

  const activeError = validationError || errorMessage;

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
            <span className="text-xs text-stone-400 font-mono">{host.mac || 'MAC não identificado'}</span>
          </div>
          <div className="text-xs text-stone-400 truncate">
            {host.kind === 'private'
              ? 'MAC Privado (Aleatório)'
              : host.vendor || 'Fabricante não identificado'}
          </div>
        </div>

        {/* Port Range Config */}
        <div className="p-4 border-b border-border-subtle space-y-3 shrink-0">
          <div className="grid grid-cols-3 gap-2.5">
            <div className="space-y-1">
              <label htmlFor="start-port-input" className="text-xs font-semibold text-stone-200">
                Porta Inicial
              </label>
              <input
                id="start-port-input"
                type="number"
                min="1"
                max="65535"
                value={startPortInput}
                disabled={isScanningPorts}
                onChange={(e) => {
                  setStartPortInput(e.target.value);
                  setValidationError(null);
                }}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="end-port-input" className="text-xs font-semibold text-stone-200">
                Porta Final
              </label>
              <input
                id="end-port-input"
                type="number"
                min="1"
                max="65535"
                value={endPortInput}
                disabled={isScanningPorts}
                onChange={(e) => {
                  setEndPortInput(e.target.value);
                  setValidationError(null);
                }}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="timeout-port-input" className="text-xs font-semibold text-stone-200">
                Timeout (ms)
              </label>
              <input
                id="timeout-port-input"
                type="number"
                min="200"
                max="5000"
                step="100"
                value={timeoutInput}
                disabled={isScanningPorts}
                onChange={(e) => {
                  setTimeoutInput(e.target.value);
                  setValidationError(null);
                }}
                className="w-full bg-stone-900 border border-stone-700/80 rounded-lg px-2.5 py-1.5 text-sm text-stone-100 font-mono focus:outline-none focus:border-stone-400 disabled:opacity-50 shadow-inner"
              />
            </div>
          </div>

          {/* Quick presets */}
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => applyPreset(1, 1024)}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              1–1024 (Padrão)
            </button>
            <button
              type="button"
              onClick={() => applyPreset(20, 445)}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              Serviços Comuns
            </button>
            <button
              type="button"
              onClick={() => applyPreset(1, 8080)}
              disabled={isScanningPorts}
              className="px-2.5 py-1 rounded-md text-xs font-medium bg-stone-800 border border-stone-700 text-stone-200 hover:text-stone-100 hover:bg-stone-700 transition-colors"
            >
              Até 8080
            </button>
          </div>

          {/* Validation or Backend Error Banner */}
          {activeError && (
            <div className="p-2.5 rounded-lg bg-red-950/35 border border-red-800/60 text-red-200 text-xs flex items-start gap-2">
              <AlertCircle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
              <span>{activeError}</span>
            </div>
          )}

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
              <span>Parar verificação</span>
            </button>
          )}
        </div>

        {/* Counter Summary & View Filter Tabs */}
        <div className="px-4 py-2.5 bg-stone-900/60 border-b border-border-subtle flex items-center justify-between gap-2 text-xs shrink-0">
          <div className="flex items-center gap-1 bg-stone-900 p-0.5 rounded-lg border border-stone-700/80">
            <button
              type="button"
              onClick={() => setFilterMode('open')}
              className={`px-2 py-1 rounded-md font-semibold transition-colors ${
                filterMode === 'open'
                  ? 'bg-sage text-stone-950'
                  : 'text-stone-300 hover:text-stone-100'
              }`}
            >
              Abertas ({openPorts.length})
            </button>
            <button
              type="button"
              onClick={() => setFilterMode('all')}
              className={`px-2 py-1 rounded-md font-semibold transition-colors ${
                filterMode === 'all'
                  ? 'bg-stone-700 text-stone-100'
                  : 'text-stone-300 hover:text-stone-100'
              }`}
            >
              Todas ({portResults.length})
            </button>
          </div>

          <div className="flex items-center gap-2 font-mono text-xs">
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
                  <span className="font-medium text-stone-200">Verificando portas TCP...</span>
                </>
              ) : (
                <span className="font-medium text-stone-300">Clique em Escanear Portas para iniciar.</span>
              )}
            </div>
          ) : displayedPorts.length === 0 ? (
            <div className="h-48 flex flex-col items-center justify-center text-center px-4 text-stone-300 text-sm gap-2">
              {isScanningPorts ? (
                <>
                  <Activity className="w-5 h-5 animate-spin text-sage mb-1" />
                  <span className="font-medium text-stone-200">Procurando portas abertas...</span>
                </>
              ) : (
                <>
                  <span className="font-medium text-stone-200">
                    Nenhuma porta aberta encontrada neste intervalo.
                  </span>
                  <span className="text-xs text-stone-400">
                    {portResults.length} portas testadas ({closedCount} fechadas, {filteredCount} filtradas).
                  </span>
                  <button
                    type="button"
                    onClick={() => setFilterMode('all')}
                    className="mt-1 px-3 py-1 rounded-md bg-stone-800 hover:bg-stone-700 border border-stone-600 text-xs font-semibold text-stone-200 transition-colors"
                  >
                    Ver todas as {portResults.length} portas
                  </button>
                </>
              )}
            </div>
          ) : (
            <div className="space-y-1.5">
              {displayedPorts.map((p) => {
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
                        <span className="text-stone-500">Serviço não listado</span>
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
