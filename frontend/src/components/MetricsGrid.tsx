import React from 'react';
import { ShieldAlert, Radio, Server, CheckCircle2, AlertCircle } from 'lucide-react';
import { SweepProgressEvent } from '../types/scanner';

interface MetricsGridProps {
  progress: SweepProgressEvent | null;
  isSweeping: boolean;
}

export const MetricsGrid: React.FC<MetricsGridProps> = ({ progress, isSweeping }) => {
  const stage = progress?.stage || 'starting';
  const percent = progress ? Math.min(100, Math.max(0, Math.round(progress.percent))) : 0;
  const total = progress?.totalHosts || 0;
  const active = progress?.activeHosts || 0;
  const icmp = progress?.icmpDiscovered || 0;
  const arp = progress?.arpDiscovered || 0;
  const offline = Math.max(0, total - active);

  const getStageDescription = () => {
    switch (stage) {
      case 'starting':
        return 'Preparando varredura e snapshot prévio...';
      case 'icmp':
        return 'Estágio 1: Varredura ICMP (Ping Sweep)';
      case 'arp':
        return 'Estágio 2: Descoberta via ARP Provocado (Hosts com Firewall)';
      case 'finished':
        return 'Varredura Concluída';
      case 'canceled':
        return 'Varredura Cancelada';
      case 'error':
        return 'Falha na Varredura';
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
            <span className="font-mono text-stone-200 font-semibold">{percent}%</span>
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
            <div className="mt-2.5 p-3 rounded-lg bg-red-950/30 border border-red-900/50 text-red-200 text-xs sm:text-sm space-y-1.5">
              <div className="font-semibold text-red-100 flex items-center gap-1.5">
                <AlertCircle className="w-4 h-4 text-red-400 shrink-0" />
                <span>Detalhes da falha:</span>
              </div>
              <p className="font-mono text-xs text-red-200/90 whitespace-pre-wrap">
                {progress.errorMessage}
              </p>
            </div>
          )}
        </div>
      )}

      {/* 5 Minimalist Metric Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
        {/* Total Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <Server className="w-3.5 h-3.5 text-stone-400" />
            <span>Total na Faixa</span>
          </div>
          <div className="text-xl font-bold font-mono text-stone-100">{total}</div>
        </div>

        {/* Active Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold">Dispositivos Ativos</div>
          <div className="text-xl font-bold font-mono text-stone-100">{active}</div>
        </div>

        {/* ICMP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <Radio className="w-3.5 h-3.5 text-sage" />
            <span>Ativos via ICMP</span>
          </div>
          <div className="text-xl font-bold font-mono text-sage">{icmp}</div>
        </div>

        {/* ARP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1">
          <div className="text-xs text-stone-300 font-semibold flex items-center gap-1.5">
            <ShieldAlert className="w-3.5 h-3.5 text-terracotta" />
            <span>Via ARP (Firewall)</span>
          </div>
          <div className="text-xl font-bold font-mono text-terracotta">{arp}</div>
        </div>

        {/* Inactive / Offline */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3.5 space-y-1 col-span-2 sm:col-span-1">
          <div className="text-xs text-stone-300 font-semibold">Silenciosos / Offline</div>
          <div className="text-xl font-bold font-mono text-stone-300">{offline}</div>
        </div>
      </div>
    </div>
  );
};
