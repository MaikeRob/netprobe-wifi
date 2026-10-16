import React from 'react';
import { ShieldAlert, Radio, Server, CheckCircle2 } from 'lucide-react';
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
          <div className="flex items-center justify-between text-xs">
            <span className="text-stone-300 font-medium flex items-center gap-2">
              {stage === 'finished' && <CheckCircle2 className="w-3.5 h-3.5 text-sage" />}
              {getStageDescription()}
            </span>
            <span className="font-mono text-stone-400">{percent}%</span>
          </div>

          <div className="h-1.5 w-full bg-stone-900 rounded-full overflow-hidden">
            <div
              className={`h-full transition-all duration-300 rounded-full ${
                stage === 'finished'
                  ? 'bg-sage'
                  : stage === 'arp'
                  ? 'bg-terracotta'
                  : 'bg-stone-300'
              }`}
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>
      )}

      {/* 5 Minimalist Metric Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
        {/* Total Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3 space-y-1">
          <div className="text-[11px] text-stone-400 font-medium flex items-center gap-1.5">
            <Server className="w-3 h-3 text-stone-500" />
            <span>Total na Faixa</span>
          </div>
          <div className="text-lg font-semibold font-mono text-stone-100">{total}</div>
        </div>

        {/* Active Hosts */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3 space-y-1">
          <div className="text-[11px] text-stone-400 font-medium">Dispositivos Ativos</div>
          <div className="text-lg font-semibold font-mono text-stone-100">{active}</div>
        </div>

        {/* ICMP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3 space-y-1">
          <div className="text-[11px] text-stone-400 font-medium flex items-center gap-1.5">
            <Radio className="w-3 h-3 text-sage" />
            <span>Ativos via ICMP</span>
          </div>
          <div className="text-lg font-semibold font-mono text-sage">{icmp}</div>
        </div>

        {/* ARP Discovered */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3 space-y-1">
          <div className="text-[11px] text-stone-400 font-medium flex items-center gap-1.5">
            <ShieldAlert className="w-3 h-3 text-terracotta" />
            <span>Via ARP (Firewall)</span>
          </div>
          <div className="text-lg font-semibold font-mono text-terracotta">{arp}</div>
        </div>

        {/* Inactive / Offline */}
        <div className="bg-surface border border-border-subtle rounded-xl p-3 space-y-1 col-span-2 sm:col-span-1">
          <div className="text-[11px] text-stone-400 font-medium">Silenciosos / Offline</div>
          <div className="text-lg font-semibold font-mono text-stone-500">{offline}</div>
        </div>
      </div>
    </div>
  );
};
