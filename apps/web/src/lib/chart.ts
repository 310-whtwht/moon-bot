// Types and helpers for the live chart (apps/api handlers/chart.go).

import type { Position } from '@/lib/bot';

export interface ChartBar {
  /** Open time, Unix seconds (UTC). */
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
}

export interface DeployedStrategy {
  type: string;
  version: string;
  enabled: boolean;
  params: Record<string, number>;
}

export interface ChartData {
  symbol: string;
  timeframe: string;
  bars: ChartBar[];
  positions: Position[];
  /** The strategy trading this symbol on this timeframe, if any. */
  strategy: DeployedStrategy | null;
}

export interface Quote {
  bid: number;
  ask: number;
  /** Unix seconds (UTC). */
  time: number;
}

export const TIMEFRAME_SECONDS: Record<string, number> = {
  '1m': 60,
  '5m': 300,
  '15m': 900,
  '30m': 1800,
  '1h': 3600,
  '4h': 14400,
  '8h': 28800,
  '12h': 43200,
  '1d': 86400,
};

export async function fetchChart(
  symbol: string,
  timeframe: string
): Promise<ChartData> {
  const query = new URLSearchParams({ symbol, timeframe });
  const response = await fetch(`/api/v1/chart?${query}`, { cache: 'no-store' });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || `API error (${response.status})`);
  }
  return body.data as ChartData;
}

/**
 * EMA of each value, seeded with the simple average of the first `period`
 * values: the same calculation as the bot (packages/core/indicator).
 */
export function ema(values: number[], period: number): (number | null)[] {
  const k = 2 / (period + 1);
  const out: (number | null)[] = [];
  let sum = 0;
  let value = 0;
  values.forEach((v, i) => {
    if (i < period) {
      sum += v;
      value = sum / period;
      out.push(i === period - 1 ? value : null);
      return;
    }
    value += k * (v - value);
    out.push(value);
  });
  return out;
}

/**
 * Applies a live quote to the bars: moves the forming bar, or opens the next
 * one. Bars up to one hour start on multiples of their length; longer ones
 * follow the broker's trading day, so they are left to the server.
 */
export function withQuote(
  bars: ChartBar[],
  quote: Quote | null,
  timeframe: string
): ChartBar[] {
  const seconds = TIMEFRAME_SECONDS[timeframe];
  const last = bars[bars.length - 1];
  if (!quote || !last || !seconds || seconds > 3600) {
    return bars;
  }
  const bucket = Math.floor(quote.time / seconds) * seconds;
  if (bucket < last.time) {
    return bars;
  }
  if (bucket === last.time) {
    const moved = {
      ...last,
      high: Math.max(last.high, quote.bid),
      low: Math.min(last.low, quote.bid),
      close: quote.bid,
    };
    return [...bars.slice(0, -1), moved];
  }
  const { bid } = quote;
  return [
    ...bars,
    { time: bucket, open: bid, high: bid, low: bid, close: bid },
  ];
}
