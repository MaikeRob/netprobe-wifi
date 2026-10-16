import React, { useState } from 'react';
import { Play, Square, Sliders, AlertTriangle } from 'lucide-react';
import { NetworkAdapter } from '../types/scanner';

interface ControlsProps {
  adapters: NetworkAdapter[];
  selectedAdapter: NetworkAdapter | null;
  onSelectAdapter: (adapter: NetworkAdapter) => void;
  cidr: string;
  onChangeCIDR: (cidr: string) => void;
  rateLimit: number;
  onChangeRateLimit: (rate: number) => void;
  timeoutMs: number;
  onChangeTimeoutMs: (timeout: number) => void;
  isSweeping: boolean;
  onStartSweep: () => void;
  onCancelSweep: () => void;
}

export const Controls: React.FC<ControlsProps> = ({
  adapters,
  selectedAdapter,
  onSelectAdapter,
  cidr,
  onChangeCIDR,
  rateLimit,
  onChangeRateLimit,
  timeoutMs,
  onChangeTimeoutMs,
  isSweeping,
  onStartSweep,
  onCancelSweep,
}) => {
  const [showAdvanced, setShowAdvanced] = useState(false);

  // Check if prefix is wider than /20 (e.g. /16)
  const isLargeRange = (() => {
    const parts = cidr.split('/');
    if (parts.length === 2) {
      const bits = parseInt(parts[1], 10);
      return !isNaN(bits) && bits > 0 && bits < 20;
    }
    return false;
  })();

  return (
    <div className="bg-surface rounded-xl border border-border-subtle p-4 space-y-3.5">
      {/* Primary Input Row */}
      <div className="grid grid-cols-1 sm:grid-cols-12 gap-3 items-end">
        {/* Network Interface Selector */}
        <div className="sm:col-span-5 space-y-1.5">
          <label className="text-xs font-medium text-stone-300">Interface de Rede</label>
          <select
            value={selectedAdapter?.name || ''}
            onChange={(e) => {
              const found = adapters.find((a) => a.name === e.target.value);
              if (found) onSelectAdapter(found);
            }}
            disabled={isSweeping}
            className="w-full bg-stone-900 border border-border-subtle rounded-lg px-3 py-2 text-xs text-stone-200 focus:outline-none focus:border-stone-500 disabled:opacity-50 transition-colors"
          >
            {adapters.map((a) => (
              <option key={a.name} value={a.name}>
                {a.name} {a.ip ? `(${a.ip})` : ''} {a.cidr ? `[${a.cidr}]` : ''}
              </option>
            ))}
          </select>
        </div>

        {/* CIDR Input Field */}
        <div className="sm:col-span-4 space-y-1.5">
          <label className="text-xs font-medium text-stone-300">Faixa de Rede (CIDR)</label>
          <input
            type="text"
            value={cidr}
            onChange={(e) => onChangeCIDR(e.target.value)}
            disabled={isSweeping}
            placeholder="192.168.1.0/24"
            className="w-full bg-stone-900 border border-border-subtle rounded-lg px-3 py-2 text-xs text-stone-200 font-mono focus:outline-none focus:border-stone-500 disabled:opacity-50 transition-colors"
          />
        </div>

        {/* Action Buttons */}
        <div className="sm:col-span-3 flex items-center gap-2">
          {!isSweeping ? (
            <button
              onClick={onStartSweep}
              className="flex-1 bg-sage hover:bg-sage-hover text-stone-950 font-medium text-xs px-4 py-2 rounded-lg flex items-center justify-center gap-1.5 transition-all shadow-sm active:scale-[0.98]"
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              <span>Iniciar</span>
            </button>
          ) : (
            <button
              onClick={onCancelSweep}
              className="flex-1 bg-stone-800 hover:bg-red-950/40 hover:text-red-300 border border-red-900/40 text-stone-300 font-medium text-xs px-4 py-2 rounded-lg flex items-center justify-center gap-1.5 transition-all active:scale-[0.98]"
            >
              <Square className="w-3.5 h-3.5 fill-current" />
              <span>Cancelar</span>
            </button>
          )}

          <button
            onClick={() => setShowAdvanced(!showAdvanced)}
            title="Opções avançadas de taxa e timeout"
            className={`p-2 rounded-lg border text-xs transition-colors ${
              showAdvanced
                ? 'bg-stone-800 border-stone-600 text-stone-200'
                : 'bg-stone-900 border-border-subtle text-stone-400 hover:text-stone-300'
            }`}
          >
            <Sliders className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Large Range Warning Banner */}
      {isLargeRange && (
        <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-amber-950/20 border border-amber-900/40 text-amber-300 text-xs">
          <AlertTriangle className="w-4 h-4 shrink-0 text-terracotta" />
          <span>
            <strong>Aviso de escala:</strong> Redes com máscara menor que /20 contêm milhares de endereços e podem levar alguns minutos para concluir.
          </span>
        </div>
      )}

      {/* Collapsible Advanced Options */}
      {showAdvanced && (
        <div className="pt-3 border-t border-border-subtle grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1">
            <div className="flex justify-between text-[11px] text-stone-400">
              <span>Taxa de Emissão (Pacotes/s)</span>
              <span className="font-mono text-stone-300">{rateLimit} pps</span>
            </div>
            <input
              type="range"
              min="10"
              max="500"
              step="10"
              value={rateLimit}
              disabled={isSweeping}
              onChange={(e) => onChangeRateLimit(parseInt(e.target.value, 10))}
              className="w-full accent-stone-400 cursor-pointer"
            />
          </div>

          <div className="space-y-1">
            <div className="flex justify-between text-[11px] text-stone-400">
              <span>Timeout de Resposta</span>
              <span className="font-mono text-stone-300">{timeoutMs} ms</span>
            </div>
            <input
              type="range"
              min="200"
              max="5000"
              step="100"
              value={timeoutMs}
              disabled={isSweeping}
              onChange={(e) => onChangeTimeoutMs(parseInt(e.target.value, 10))}
              className="w-full accent-stone-400 cursor-pointer"
            />
          </div>
        </div>
      )}
    </div>
  );
};
