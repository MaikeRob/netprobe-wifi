import React, { useState } from 'react';
import { ShieldCheck, ShieldAlert, Activity, RefreshCw, Copy, Check } from 'lucide-react';
import { Capabilities } from '../types/scanner';

interface HeaderProps {
  capabilities: Capabilities | null;
  isSweeping: boolean;
  onRefreshCapabilities?: () => void;
}

const SYSCTL_CMD = 'sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"';

export const Header: React.FC<HeaderProps> = ({
  capabilities,
  isSweeping,
  onRefreshCapabilities,
}) => {
  const [copied, setCopied] = useState(false);

  const handleCopyCommand = async () => {
    try {
      await navigator.clipboard.writeText(SYSCTL_CMD);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback if clipboard API is unavailable
    }
  };

  return (
    <div className="space-y-2.5 pb-3.5 border-b border-border-subtle">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-stone-100 flex items-center gap-2">
            Mapeamento de Rede Local
            {isSweeping && (
              <span className="flex items-center gap-1.5 text-xs font-normal text-sage px-2 py-0.5 rounded-full bg-sage-subtle border border-sage/20 animate-pulse">
                <Activity className="w-3 h-3" />
                Varrendo...
              </span>
            )}
          </h1>
          <p className="text-xs sm:text-sm text-stone-300 mt-0.5">
            Descoberta em tempo real via ICMP Echo e ARP provocado pelo kernel.
          </p>
        </div>

        {/* System Capabilities Badge */}
        {capabilities && (
          <div className="flex items-center gap-2 text-xs sm:text-sm">
            {capabilities.CanSendICMP ? (
              <div className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-stone-900 border border-stone-700/80 text-stone-100 font-medium">
                <ShieldCheck className="w-4 h-4 text-sage" />
                <span>Privilégios ICMP Ativos</span>
              </div>
            ) : (
              <div
                className="flex items-center gap-1.5 px-3 py-1 rounded-full bg-amber-950/40 border border-amber-800/60 text-amber-200 font-medium"
                title={capabilities.Reason}
              >
                <ShieldAlert className="w-4 h-4 text-amber-400" />
                <span>ICMP Restrito (Modo ARP Ativo)</span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Actionable Linux ICMP Permission Guidance Banner */}
      {capabilities && !capabilities.CanSendICMP && (
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2.5 px-3.5 py-2.5 rounded-lg bg-amber-950/30 border border-amber-800/60 text-amber-200 text-xs">
          <div className="space-y-1">
            <div className="font-semibold text-amber-100 flex items-center gap-1.5">
              <ShieldAlert className="w-4 h-4 text-amber-400 shrink-0" />
              <span>
                Socket ICMP desabilitado no kernel Linux — a varredura utilizará apenas o Estágio 2 (ARP Provocado).
              </span>
            </div>
            <div className="text-amber-200/90">
              Para demonstrar pacotes <strong className="text-amber-100">ICMP Echo Request/Reply no Wireshark (Elemento 1)</strong>, habilite sockets ICMP não privilegiados no terminal:
              <code className="ml-1.5 px-1.5 py-0.5 rounded bg-stone-900/90 border border-amber-800/50 font-mono text-amber-100 select-all">
                {SYSCTL_CMD}
              </code>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <button
              type="button"
              onClick={handleCopyCommand}
              className="px-2.5 py-1.5 rounded-md bg-stone-900 hover:bg-stone-800 border border-amber-700/60 text-amber-100 font-medium flex items-center gap-1.5 transition-colors"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-sage" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copied ? 'Copiado' : 'Copiar comando'}</span>
            </button>
            {onRefreshCapabilities && (
              <button
                type="button"
                onClick={onRefreshCapabilities}
                className="px-2.5 py-1.5 rounded-md bg-amber-900/40 hover:bg-amber-900/60 border border-amber-700/70 text-amber-100 font-semibold flex items-center gap-1.5 transition-colors"
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>Verificar novamente</span>
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
