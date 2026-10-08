// Stored price history and the downloads of more (apps/api handlers/marketdata.go).

import type { Instrument } from '@/lib/bot';

/** The stored history of one symbol and timeframe. */
export interface Coverage {
  symbol: string;
  timeframe: string;
  bid_bars: number;
  ask_bars: number;
  first: string;
  last: string;
}

export type ImportStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled';

/** One request to download history. */
export interface DataImport {
  id: string;
  symbol: string;
  timeframe: string;
  from_date: string;
  status: ImportStatus;
  bars_stored: number;
  /** Where the download has got to, e.g. "ASK 2025-06-01". */
  progress: string;
  error: string | null;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
}

export interface MarketData {
  instruments: Instrument[];
  coverage: Coverage[];
  imports: DataImport[];
  /** First day the broker serves bars for (YYYY-MM-DD). */
  history_start: string;
}

/** Timeframes that can be downloaded and backtested, shortest first. */
export const DATA_TIMEFRAMES = [
  { value: '1m', label: '1分足' },
  { value: '5m', label: '5分足' },
  { value: '15m', label: '15分足' },
  { value: '30m', label: '30分足' },
  { value: '1h', label: '1時間足' },
  { value: '4h', label: '4時間足' },
  { value: '1d', label: '日足' },
];

export const timeframeLabel = (tf: string) =>
  DATA_TIMEFRAMES.find(t => t.value === tf)?.label ?? tf;

const timeframeOrder = (tf: string) => {
  const i = DATA_TIMEFRAMES.findIndex(t => t.value === tf);
  return i === -1 ? DATA_TIMEFRAMES.length : i;
};

/** Orders coverage by symbol, then from the shortest timeframe. */
export function sortCoverage(coverage: Coverage[]): Coverage[] {
  return [...coverage].sort(
    (a, b) =>
      a.symbol.localeCompare(b.symbol) ||
      timeframeOrder(a.timeframe) - timeframeOrder(b.timeframe)
  );
}

/**
 * A backtest needs both BID and ASK bars. They are downloaded one after the
 * other, so a count that differs means the download is still going or failed.
 */
export const usable = (c: Coverage) =>
  c.bid_bars > 0 && c.ask_bars === c.bid_bars;

async function parse<T>(response: Response): Promise<T> {
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || `API error (${response.status})`);
  }
  return body.data as T;
}

export async function fetchMarketData(): Promise<MarketData> {
  return parse<MarketData>(
    await fetch('/api/v1/market-data', { cache: 'no-store' })
  );
}

/** Queues a download of BID and ASK bars from `from` (YYYY-MM-DD) to now. */
export async function requestImport(
  symbol: string,
  timeframe: string,
  from: string
): Promise<void> {
  await parse(
    await fetch('/api/v1/market-data/imports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ symbol, timeframe, from }),
    })
  );
}

/** Withdraws a download that has not started. */
export async function cancelImport(id: string): Promise<void> {
  await parse(
    await fetch(`/api/v1/market-data/imports/${id}`, { method: 'DELETE' })
  );
}
