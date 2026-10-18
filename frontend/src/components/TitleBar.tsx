import React from 'react';
import { Minus, Square, X, Wifi } from 'lucide-react';
import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime/runtime';

export const TitleBar: React.FC = () => {
  return (
    <header className="h-10 w-full bg-background border-b border-border-subtle flex items-center justify-between px-3 select-none wails-drag shrink-0 z-50">
      {/* Brand & Draggable Zone */}
      <div className="flex items-center gap-2.5">
        <div className="w-5 h-5 rounded-md bg-stone-800 flex items-center justify-center text-stone-300">
          <Wifi className="w-3.5 h-3.5" />
        </div>
        <span className="text-xs font-semibold tracking-wide text-stone-100">NetProbe Wi-Fi</span>
        <span className="text-xs text-stone-300 font-mono px-2 py-0.5 rounded bg-stone-900 border border-stone-700/80">v1.0</span>
      </div>

      {/* Window Controls (No-Drag) */}
      <div className="flex items-center gap-1 wails-no-drag">
        <button
          onClick={() => WindowMinimise()}
          title="Minimizar"
          className="w-7 h-7 rounded flex items-center justify-center text-stone-300 hover:text-stone-100 hover:bg-stone-800 transition-colors"
        >
          <Minus className="w-3.5 h-3.5" />
        </button>
        <button
          onClick={() => WindowToggleMaximise()}
          title="Maximizar"
          className="w-7 h-7 rounded flex items-center justify-center text-stone-300 hover:text-stone-100 hover:bg-stone-800 transition-colors"
        >
          <Square className="w-3 h-3" />
        </button>
        <button
          onClick={() => Quit()}
          title="Fechar"
          className="w-7 h-7 rounded flex items-center justify-center text-stone-300 hover:text-red-300 hover:bg-red-500/20 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
    </header>
  );
};
