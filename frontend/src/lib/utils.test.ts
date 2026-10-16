import { describe, it, expect } from 'vitest';
import { formatDuration, cn } from './utils';

describe('utils', () => {
  it('formatDuration handles zero', () => {
    expect(formatDuration(0)).toBe('0 ms');
  });

  it('formatDuration handles microseconds', () => {
    expect(formatDuration(500_000)).toBe('500.0 µs');
  });

  it('formatDuration handles milliseconds', () => {
    expect(formatDuration(5_500_000)).toBe('5.5 ms');
  });

  it('cn merges tailwind classes cleanly', () => {
    expect(cn('p-2', 'p-4')).toBe('p-4');
    expect(cn('text-stone-100', true && 'font-bold')).toBe('text-stone-100 font-bold');
  });
});
