import React, { useState, useMemo } from 'react';
import { Search, ShieldAlert, Radio, Terminal, Download } from 'lucide-react';
import { DiscoveredHost } from '../types/scanner';
import { formatDuration } from '../lib/utils';

interface HostTableProps {
  hosts: DiscoveredHost[];
  onSelectHostForPortScan: (host: DiscoveredHost) => void;
}

export const HostTable: React.FC<HostTableProps> = ({ hosts, onSelectHostForPortScan }) => {
  const [searchTerm, setSearchTerm] = useState('');

  const filteredHosts = useMemo(() => {
    if (!searchTerm.trim()) return hosts;
    const term = searchTerm.toLowerCase();
    return hosts.filter(
      (h) =>
        h.ip.toLowerCase().includes(term) ||
        h.mac.toLowerCase().includes(term) ||
        h.vendor.toLowerCase().includes(term) ||
        h.discoveredBy.toLowerCase().includes(term)
    );
  }, [hosts, searchTerm]);

  const exportCSV = () => {
    if (hosts.length === 0) return;
    const headers = ['IP', 'MAC', 'Fabricante', 'Tipo', 'RTT (ms)', 'Metodo'];
    const rows = hosts.map((h) => [
      h.ip,
      h.mac,
      `"${h.vendor.replace(/"/g, '""')}"`,
      h.kind,
      (h.rtt / 1_000_000).toFixed(2),
      `"${h.discoveredBy}"`,
    ]);
    const csvContent = 'data:text/csv;charset=utf-8,' + [headers.join(','), ...rows.map((r) => r.join(','))].join('\n');
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', `netprobe_hosts_${new Date().toISOString().slice(0, 10)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="bg-surface rounded-xl border border-border-subtle flex flex-col flex-1 min-h-0 overflow-hidden">
      {/* Table Header Controls */}
      <div className="p-3 border-b border-border-subtle flex items-center justify-between gap-3 shrink-0">
        <div className="relative flex-1 max-w-xs">
          <Search className="w-3.5 h-3.5 text-stone-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Filtrar por IP, MAC ou Fabricante..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full bg-stone-900 border border-border-subtle rounded-lg pl-8 pr-3 py-1.5 text-xs text-stone-200 placeholder:text-stone-500 focus:outline-none focus:border-stone-500 transition-colors"
          />
        </div>

        <div className="flex items-center gap-2">
          <span className="text-xs text-stone-400 font-mono">
            {filteredHosts.length} {filteredHosts.length === 1 ? 'dispositivo' : 'dispositivos'}
          </span>
          {hosts.length > 0 && (
            <button
              onClick={exportCSV}
              title="Exportar tabela para CSV"
              className="p-1.5 rounded-lg border border-border-subtle bg-stone-900 text-stone-400 hover:text-stone-200 transition-colors"
            >
              <Download className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Table Body (Scrollable) */}
      <div className="flex-1 overflow-auto">
        {filteredHosts.length === 0 ? (
          <div className="h-48 flex flex-col items-center justify-center text-stone-500 text-xs gap-1.5">
            <span>Nenhum dispositivo encontrado na varredura.</span>
            <span className="text-[11px] text-stone-600">
              Inicie a varredura para visualizar dispositivos ativos em tempo real.
            </span>
          </div>
        ) : (
          <table className="w-full text-left text-xs border-collapse">
            <thead className="bg-stone-900/50 text-stone-400 sticky top-0 z-10 border-b border-border-subtle">
              <tr>
                <th className="py-2.5 px-3 font-medium">Endereço IP</th>
                <th className="py-2.5 px-3 font-medium">Endereço MAC</th>
                <th className="py-2.5 px-3 font-medium">Fabricante</th>
                <th className="py-2.5 px-3 font-medium">Latência</th>
                <th className="py-2.5 px-3 font-medium">Método</th>
                <th className="py-2.5 px-3 font-medium text-right">Ação</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {filteredHosts.map((host) => {
                const isARP = host.discoveredBy === 'Descoberto via ARP';
                const isPrivate = host.kind === 'private';

                return (
                  <tr
                    key={host.ip}
                    className="hover:bg-surface-elevated/40 transition-colors group"
                  >
                    {/* IP Address */}
                    <td className="py-2.5 px-3 font-mono font-medium text-stone-200">
                      {host.ip}
                    </td>

                    {/* MAC Address */}
                    <td className="py-2.5 px-3 font-mono text-stone-300">
                      {host.mac || <span className="text-stone-600 italic">Desconhecido</span>}
                    </td>

                    {/* Vendor / Kind */}
                    <td className="py-2.5 px-3">
                      {isPrivate ? (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium bg-stone-800 text-stone-300 border border-stone-700/60">
                          MAC Randômico / Privado
                        </span>
                      ) : host.vendor ? (
                        <span className="text-stone-200 truncate max-w-xs block" title={host.vendor}>
                          {host.vendor}
                        </span>
                      ) : (
                        <span className="text-stone-600 italic">Não identificado</span>
                      )}
                    </td>

                    {/* Latency */}
                    <td className="py-2.5 px-3 font-mono text-stone-400">
                      {host.responded ? formatDuration(host.rtt) : <span className="text-stone-600">-</span>}
                    </td>

                    {/* Discovery Status Badge */}
                    <td className="py-2.5 px-3">
                      {isARP ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-amber-950/30 text-amber-300 border border-amber-900/40">
                          <ShieldAlert className="w-3 h-3 text-terracotta shrink-0" />
                          <span>Descoberto via ARP</span>
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium bg-stone-900 text-stone-300 border border-stone-800">
                          <Radio className="w-3 h-3 text-sage shrink-0" />
                          <span>Ativo via ICMP</span>
                        </span>
                      )}
                    </td>

                    {/* Action: Port Scan */}
                    <td className="py-2.5 px-3 text-right">
                      <button
                        onClick={() => onSelectHostForPortScan(host)}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded bg-stone-900 hover:bg-stone-800 text-stone-300 border border-border-subtle text-[11px] font-medium transition-colors"
                      >
                        <Terminal className="w-3 h-3" />
                        <span>Portas</span>
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
};
