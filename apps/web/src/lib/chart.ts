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

/** Where the deployed strategy would have traded: a replay, not a fill. */
export interface ChartSignal {
  /** Open time of the bar it acts on, Unix seconds (UTC). */
  time: number;
  kind: 'buy' | 'sell' | 'exit' | 'stop';
  price: number;
}

/** What the strategy decided on one closed bar (recorded by the bot). */
export interface BarDecision {
  /** Open time of the bar, ISO (UTC). */
  bar_time: string;
  close: number;
  action: 'HOLD' | 'ENTER_LONG' | 'ENTER_SHORT' | 'EXIT';
  /** Side held when the bar was judged. */
  holding: '' | 'BUY' | 'SELL';
  /** Indicator values the strategy saw, e.g. "fast=1 slow=2 diff=-1 atr=0.1". */
  detail: string;
  /**
   * What became of the signal: "kind: message" parts joined by " / "
   * (opened, closed, skipped, rejected, error). Empty when there was none.
   */
  result: string;
  decided_at: string;
}

export interface ChartData {
  symbol: string;
  timeframe: string;
  bars: ChartBar[];
  positions: Position[];
  /** The strategy trading this symbol on this timeframe, if any. */
  strategy: DeployedStrategy | null;
  /** Null when no strategy trades this symbol and timeframe. */
  signals: ChartSignal[] | null;
  /** Newest first. */
  decisions: BarDecision[];
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

const RESULT_KINDS: Record<string, string> = {
  opened: '新規約定',
  closed: '決済',
  skipped: '発注せず',
  rejected: '発注せず',
  pending: '指値で待機',
  cancelled: '指値を取消',
  partial: '一部約定',
  error: 'エラー',
};

/** Why an order was held back, for the reasons the bot is known to give. */
const RESULT_REASONS: [RegExp, (m: RegExpMatchArray) => string][] = [
  [
    /daily_loss: lost (\d+) JPY today, limit (\d+)/,
    m =>
      `1日の損失上限に達しています（今日の損失 ${m[1]} 円 / 上限 ${m[2]} 円）`,
  ],
  [
    /weekly_loss: lost (\d+) JPY this week, limit (\d+)/,
    m =>
      `1週間の損失上限に達しています（今週の損失 ${m[1]} 円 / 上限 ${m[2]} 円）`,
  ],
  [
    /max_open_positions: (\d+) open, limit (\d+)/,
    m => `同時に持てる建玉の上限です（保有 ${m[1]} 件 / 上限 ${m[2]} 件）`,
  ],
  [/max_units: /, () => '1建玉の数量上限を超えています'],
  [/margin: /, () => '証拠金が足りません'],
  [/entry blocked: /, () => 'Kill Switch で新規の発注を止めています'],
  [
    /spread ([\d.]+) is wider than the limit ([\d.]+)/,
    m => `スプレッドが広すぎます（${m[1]} / 上限 ${m[2]}）`,
  ],
  [
    /limit (BUY|SELL) [\d.]+ \S+ @ ([\d.]+), waiting up to (\S+)/,
    m => `${m[1] === 'BUY' ? '買い' : '売り'} ${m[2]} で最大 ${m[3]} 待ちます`,
  ],
  [
    /@ ([\d.]+): not filled within (\S+)/,
    m => `${m[1]} に届かず、${m[2]} で取り消しました`,
  ],
  [
    /@ ([\d.]+): withdrawn before it filled/,
    m => `${m[1]} の注文を、約定前に取り下げました`,
  ],
  [
    /filled ([\d.]+) of ([\d.]+)/,
    m => `${m[2]} のうち ${m[1]} だけ約定しました`,
  ],
  [
    /stale signal/,
    () => '足の確定から時間が経ちすぎています（bot の停止やデータの遅れ）',
  ],
  [/market /, () => '市場が閉まっています'],
  [/deployment disabled/, () => '割り当てが無効です'],
  [/already acted on/, () => 'この判断は処理済みです'],
  [/hard cap/, () => '本番口座の数量の安全上限を超えています'],
  [/valid stop-loss/, () => '損切りの指定がありません'],
];

/** Turns a decision's result into lines a person can read; raw text is kept as a fallback. */
export function describeResult(result: string): string[] {
  if (!result) {
    return [];
  }
  return result.split(' / ').map(part => {
    const [kind = '', ...rest] = part.split(': ');
    const message = rest.join(': ');
    const label = RESULT_KINDS[kind] ?? kind;
    if (kind === 'opened' || kind === 'closed') {
      return `${label}（${message}）`;
    }
    for (const [pattern, describe] of RESULT_REASONS) {
      const match = message.match(pattern);
      if (match) {
        return `${label}: ${describe(match)}`;
      }
    }
    return `${label}: ${message}`;
  });
}
