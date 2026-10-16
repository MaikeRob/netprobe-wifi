import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDuration(ns: number): string {
  if (ns === 0) return '0 ms';
  const ms = ns / 1_000_000;
  if (ms < 1) {
    return `${(ns / 1_000).toFixed(1)} µs`;
  }
  return `${ms.toFixed(1)} ms`;
}
