// Types and helpers for strategies and backtests produced by packages/core.

export interface ParamSpec {
  name: string;
  label: string;
  description: string;
  integer: boolean;
  default: number;
  min: number;
  max: number;
}

export interface StrategyType {
  type: string;
  name: string;
  description: string;
  params: ParamSpec[];
}

export interface StrategyVersion {
  id: string;
  package_id: string;
  version: string;
  code: string;
  description?: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  /** Registered strategy type, or "" for legacy versions. */
  strategy_type: string;
  params: Record<string, number> | null;
}

export interface BacktestTrade {
  side: 'LONG' | 'SHORT';
  entry_time: string;
  entry_price: number;
  exit_time: string;
  exit_price: number;
  units: number;
  fees: number;
  pnl: number;
  entry_reason: string;
  exit_reason: 'signal' | 'stop' | 'end';
}

export interface EquityPoint {
  time: string;
  equity: number;
}

export interface BacktestMetrics {
  initial_balance: number;
  final_equity: number;
  net_profit: number;
  total_return: number;
  cagr: number;
  max_drawdown: number;
  sharpe: number;
  num_trades: number;
  win_rate: number;
  profit_factor: number;
  avg_trade: number;
  sqn: number;
  total_fees: number;
  exposure: number;
}

export interface BacktestResult {
  strategy: string;
  params: Record<string, number>;
  from: string;
  to: string;
  bars: number;
  skipped_entries: number;
  trades: BacktestTrade[];
  equity: EquityPoint[];
  metrics: BacktestMetrics;
}

export interface BacktestSpec {
  strategy_version_id: string;
  strategy_type: string;
  strategy_params: Record<string, number>;
  broker: string;
  symbol: string;
  timeframe: string;
  units: number;
  initial_balance: number;
}

export interface Backtest {
  id: string;
  name: string;
  strategy_id: string;
  symbols: string[];
  start_date: string;
  end_date: string;
  parameters: unknown;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  results: unknown;
  error?: string | null;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
}

function parseJSON(value: unknown): unknown {
  if (typeof value === 'string') {
    try {
      return JSON.parse(value);
    } catch {
      return null;
    }
  }
  return value;
}

/** Returns the result if it has the shape produced by the engine (legacy rows return null). */
export function parseResult(value: unknown): BacktestResult | null {
  const v = parseJSON(value) as Partial<BacktestResult> | null;
  if (!v || typeof v !== 'object' || !v.metrics || !Array.isArray(v.trades)) {
    return null;
  }
  return v as BacktestResult;
}

export function parseSpec(value: unknown): BacktestSpec | null {
  const v = parseJSON(value) as Partial<BacktestSpec> | null;
  if (!v || typeof v !== 'object' || !v.strategy_type) {
    return null;
  }
  return v as BacktestSpec;
}

export const TIMEFRAMES = [
  { value: '15m', label: '15分足' },
  { value: '30m', label: '30分足' },
  { value: '1h', label: '1時間足' },
  { value: '4h', label: '4時間足' },
  { value: '1d', label: '日足' },
];

export const STATUS_LABELS: Record<string, string> = {
  pending: '待機中',
  running: '実行中',
  completed: '完了',
  failed: '失敗',
  cancelled: 'キャンセル',
};

export const EXIT_REASON_LABELS: Record<string, string> = {
  signal: 'シグナル',
  stop: '損切り',
  end: '期間終了',
};

export function formatYen(v: number): string {
  return `${v >= 0 ? '' : '-'}¥${Math.abs(v).toLocaleString('ja-JP', { maximumFractionDigits: 0 })}`;
}

export function formatPercent(fraction: number, signed = false): string {
  const pct = fraction * 100;
  const sign = signed && pct > 0 ? '+' : '';
  return `${sign}${pct.toFixed(2)}%`;
}

export function formatNumber(v: number, digits = 2): string {
  return v.toLocaleString('ja-JP', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

export function formatParams(
  params: Record<string, number> | null | undefined
): string {
  if (!params) {
    return '';
  }
  return Object.entries(params)
    .map(([k, v]) => `${k}=${v}`)
    .join(' ');
}
