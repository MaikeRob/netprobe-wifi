import React from 'react';
import { ShieldAlert, Radio, Server, CheckCircle2, AlertCircle, Wifi, MinusCircle } from 'lucide-react';
import { SweepProgressEvent } from '../types/scanner';

interface MetricsGridProps {
  progress: SweepProgressEvent | null;
  isSweeping: boolean;
}

export const MetricsGrid: React.FC<MetricsGridProps> = ({ progress, isSweeping }) => {
  const stage = progress?.stage || 'starting';
  const percent = progress ? Math.min(100, Math.max(0, Math.round(progress.percent))) : 0;
  const total = progress?.totalHosts || 0;
  const scanned = progress?.scannedHosts || 0;
  const active = progress?.activeHosts || 0;
  const icmp = progress?.icmpDiscovered || 0;
  const arp = progress?.arpDiscovered || 0;
  const offline =
    stage === 'icmp' || stage === 'starting'
      ? Math.max(0, scanned - active)
      : Math.max(0, total - active);

  const getStageDescription = () => {
    switch (stage) {
      case 'starting':
        return 'Preparando varredura da sub-rede...';
      case 'icmp':
        return 'Verificando dispositivos via Ping (ICMP)...';
      case 'arp':
        return 'Sondando dispositivos silenciosos via ARP...';
      case 'finished':
        return 'Varredura concluída';
      case 'canceled':
        return 'Varredura interrompida';
      case 'error':
        return 'Não foi possível concluir a varredura';
      default:
        return 'Pronto para iniciar';
    }
  };

  return (
    <div className="space-y-3">
      {/* Progress Bar Container */}
      {(isSweeping || progress) && (
        <div className="bg-surface rounded-xl border border-border-subtle p-3.5 space-y-2">
          <div className="flex items-center justify-between text-xs sm:text-sm">
            <span className="text-stone-100 font-medium flex items-center gap-2">
              {stage === 'finished' && <CheckCircle2 className="w-4 h-4 text-sage" />}
              {stage === 'error' && <AlertCircle className="w-4 h-4 text-red-400" />}
              {getStageDescription()}
            </span>
            <div className="flex items-center gap-2.5 font-mono">
              {total > 0 && (
                <span className="text-xs text-stone-400">
                  {scanned}/{total} verificados
                </span>
              )}
              <span className="text-stone-200 font-semibold">{percent}%</span>
            </div>
          </div>

          <div className="h-2 w-full bg-stone-900 rounded-full overflow-hidden border border-border-subtle">
            <div
              className={`h-full transition-all duration-300 rounded-full ${
                stage === 'finished'
                  ? 'bg-sage'
                  : stage === 'arp'
                  ? 'bg-terracotta'
                  : stage === 'error'
                  ? 'bg-red-500'
                  : 'bg-stone-300'
              }`}
              style={{ width: `${percent}%` }}
            />
          </div>

          {stage === 'error' && progress?.errorMessage && (
            <div className="mt-2.5 p-3 rounded-lg bg-red-950/30 border border-red-900/50 text-red-200 text-xs sm:text-sm flex items-start gap-2">
              <AlertCircle className="w-4 h-4 text-red-400 shrink-0 mt-0.5" />
              <span className="text-red-100 leading-relaxed">
                {progress.errorMessage}
              </span>
            </div>
          )}
        </div>
      )}

      {/* 5 Minimalist Metric Cards with uniform visual alignment */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
        {/* Total Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <Server className="w-3.5 h-3.5 text-stone-400 shrink-0" />
            <span>Total na Faixa</span>
          </div>
          <div className="text-xl font-bold font-mono text-stone-100">{total}</div>
        </div>

        {/* Active Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <Wifi className="w-3.5 h-3.5 text-stone-300 shrink-0" />
            <span>Dispositivos Ativos</span>
          </div>
          <div className="text-xl font-bold font-mono text-stone-100">{active}</div>
        </div>

        {/* ICMP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <Radio className="w-3.5 h-3.5 text-sage shrink-0" />
            <span>Ativos via ICMP</span>
          </div>
          <div className="text-xl font-bold font-mono text-sage">{icmp}</div>
        </div>

        {/* ARP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <ShieldAlert className="w-3.5 h-3.5 text-terracotta shrink-0" />
            <span>Via ARP (Firewall)</span>
          </div>
          <div className="text-xl font-bold font-mono text-terracotta">{arp}</div>
        </div>

        {/* Inactive / Offline */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1 col-span-2 sm:col-span-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <MinusCircle className="w-3.5 h-3.5 text-stone-500 shrink-0" />
            <span>Silenciosos / Offline</span>
          </div>
          <div className="text-xl font-bold font-mono text-stone-300">{offline}</div>
        </div>
      </div>
    </div>
  );
};
