import React from 'react';
import { ShieldCheck, ShieldAlert, Activity } from 'lucide-react';
import { Capabilities } from '../types/scanner';

interface HeaderProps {
  capabilities: Capabilities | null;
  isSweeping: boolean;
}

export const Header: React.FC<HeaderProps> = ({ capabilities, isSweeping }) => {
  return (
    <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pb-4 border-b border-border-subtle">
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
        <p className="text-xs text-stone-400 mt-0.5">
          Descoberta em tempo real via ICMP Echo e ARP provocado pelo kernel.
        </p>
      </div>

      {/* System Capabilities Badge */}
      {capabilities && (
        <div className="flex items-center gap-2 text-xs">
          {capabilities.CanSendICMP ? (
            <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-stone-900 border border-stone-800 text-stone-300">
              <ShieldCheck className="w-3.5 h-3.5 text-sage" />
              <span>Privilégios ICMP Ativos</span>
            </div>
          ) : (
            <div
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-amber-950/30 border border-amber-900/50 text-amber-300"
              title={capabilities.Reason}
            >
              <ShieldAlert className="w-3.5 h-3.5 text-terracotta" />
              <span>ICMP Restrito (Descoberta via ARP ativa)</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
