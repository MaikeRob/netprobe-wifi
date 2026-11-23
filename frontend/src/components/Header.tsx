import React, { useState, useRef, useEffect } from 'react';
import { ShieldCheck, ShieldAlert, Activity, RefreshCw, Copy, Check, ChevronDown, X } from 'lucide-react';
import { Capabilities } from '../types/scanner';

interface HeaderProps {
  capabilities: Capabilities | null;
  isSweeping: boolean;
  onRefreshCapabilities?: () => void;
}

const ENABLE_PING_CMD = 'sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"';

export const Header: React.FC<HeaderProps> = ({
  capabilities,
  isSweeping,
  onRefreshCapabilities,
}) => {
  const [showHelp, setShowHelp] = useState(false);
  const [copied, setCopied] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showHelp) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setShowHelp(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setShowHelp(false);
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [showHelp]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(ENABLE_PING_CMD);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard API unavailable
    }
  };

  return (
    <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pb-3.5 border-b border-border-subtle relative">
      <div>
        <h1 className="text-lg font-semibold tracking-tight text-stone-100 flex items-center gap-2.5">
          <span>Dispositivos na Rede</span>
          {isSweeping && (
            <span className="inline-flex items-center gap-1.5 text-xs font-medium text-sage px-2.5 py-0.5 rounded-full bg-sage-subtle border border-sage/25">
              <Activity className="w-3 h-3 animate-pulse" />
              <span>Buscando dispositivos...</span>
            </span>
          )}
        </h1>
        <p className="text-xs sm:text-sm text-stone-400 mt-0.5">
          Identifique aparelhos conectados ao seu Wi-Fi e verifique portas abertas.
        </p>
      </div>

      {/* System Capabilities Status Pill */}
      {capabilities && (
        <div className="relative" ref={popoverRef}>
          {capabilities.CanSendICMP ? (
            <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-stone-900 border border-stone-700/80 text-stone-200 text-xs font-medium">
              <ShieldCheck className="w-3.5 h-3.5 text-sage" />
              <span>Busca completa ativa</span>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setShowHelp((prev) => !prev)}
              className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-amber-950/40 hover:bg-amber-950/60 border border-amber-800/60 text-amber-200 text-xs font-medium transition-colors"
              title="Veja como ativar a medição de latência por Ping"
            >
              <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
              <span>Ping desativado</span>
              <span className="text-amber-300/90 underline decoration-amber-400/50 underline-offset-2 ml-0.5">
                Como ativar
              </span>
              <ChevronDown className={`w-3.5 h-3.5 text-amber-400 transition-transform ${showHelp ? 'rotate-180' : ''}`} />
            </button>
          )}

          {/* On-Demand Clean Popover (zero developer/compilation jargon) */}
          {!capabilities.CanSendICMP && showHelp && (
            <div className="absolute right-0 mt-2 w-80 sm:w-96 rounded-xl bg-surface-elevated border border-stone-700/90 shadow-2xl p-4 z-40 text-xs text-stone-200 space-y-3">
              <div className="flex items-start justify-between gap-2">
                <div className="font-semibold text-stone-100 text-sm">
                  Ativar verificação por Ping
                </div>
                <button
                  type="button"
                  onClick={() => setShowHelp(false)}
                  aria-label="Fechar"
                  className="p-1 rounded-md text-stone-400 hover:text-stone-100 hover:bg-stone-800 transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>

              <p className="text-stone-300 leading-relaxed">
                O Linux restringiu o envio de Ping por falta de permissão. Os dispositivos continuam sendo encontrados normalmente via ARP, apenas sem exibir o tempo de resposta (latência).
              </p>

              <div className="space-y-1.5 pt-1">
                <div className="text-xs font-medium text-stone-300">
                  Para liberar o Ping no sistema, execute no terminal:
                </div>
                <div className="flex items-center justify-between gap-2 p-2 rounded-lg bg-stone-900 border border-stone-800">
                  <code className="font-mono text-[11px] text-stone-200 truncate select-all">
                    {ENABLE_PING_CMD}
                  </code>
                  <button
                    type="button"
                    onClick={handleCopy}
                    className="px-2.5 py-1 rounded bg-stone-800 hover:bg-stone-700 text-stone-100 font-medium flex items-center gap-1 shrink-0 transition-colors"
                  >
                    {copied ? <Check className="w-3 h-3 text-sage" /> : <Copy className="w-3 h-3" />}
                    <span>{copied ? 'Copiado' : 'Copiar'}</span>
                  </button>
                </div>
              </div>

              {onRefreshCapabilities && (
                <div className="pt-2 border-t border-border-subtle flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onRefreshCapabilities();
                      setShowHelp(false);
                    }}
                    className="px-3 py-1.5 rounded-lg bg-sage hover:bg-sage-hover text-stone-950 font-semibold flex items-center gap-1.5 transition-colors"
                  >
                    <RefreshCw className="w-3.5 h-3.5" />
                    <span>Verificar novamente</span>
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
