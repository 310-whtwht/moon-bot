// Types and calls for the bot status / control API (apps/api handlers/bot.go).

export interface BotHeartbeat {
  instance: string;
  started_at: string;
  last_seen_at: string;
  poll_interval_seconds: number;
  runners: number;
  alive: boolean;
}

export interface KillSwitch {
  scope: string;
  active: boolean;
  close_positions: boolean;
  reason: string | null;
  updated_by: string | null;
  updated_at: string;
}

export interface Deployment {
  id: string;
  name: string;
  strategy_id: string;
  strategy_name: string;
  active_version: string | null;
  broker: string;
  account_id: string;
  symbol: string;
  timeframe: string;
  units: number;
  enabled: boolean;
}

export interface Position {
  id: string;
  deployment_id: string | null;
  broker: string;
  account_id: string;
  symbol: string;
  side: 'buy' | 'sell';
  quantity: number;
  open_price: number;
  stop_price: number | null;
  /** Set when the stop is also held at the broker (works while the bot is down). */
  stop_order_id: string | null;
  close_price: number | null;
  realized_pnl: number | null;
  fees: number;
  status: 'open' | 'closed';
  opened_at: string;
  closed_at: string | null;
}

export interface PnlSummary {
  since: string;
  pnl: number;
  closed: number;
}

export interface BotStatus {
  now: string;
  bot_alive: boolean;
  heartbeats: BotHeartbeat[];
  kill_switches: KillSwitch[];
  deployments: Deployment[];
  open_positions: Position[];
  closed_positions: Position[];
  daily: PnlSummary;
  weekly: PnlSummary;
}

async function parse<T>(response: Response): Promise<T> {
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || `API error (${response.status})`);
  }
  return body.data as T;
}

export async function fetchBotStatus(): Promise<BotStatus> {
  return parse<BotStatus>(
    await fetch('/api/v1/bot/status', { cache: 'no-store' })
  );
}

export async function setKillSwitch(
  scope: string,
  active: boolean,
  closePositions: boolean,
  reason: string
): Promise<KillSwitch[]> {
  return parse<KillSwitch[]>(
    await fetch(`/api/v1/kill-switch/${scope}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        active,
        close_positions: closePositions,
        reason: reason || null,
      }),
    })
  );
}

export async function setDeploymentEnabled(
  id: string,
  enabled: boolean
): Promise<void> {
  await parse(
    await fetch(`/api/v1/deployments/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    })
  );
}

/** An instrument a deployment may trade (JPY-quoted pairs). */
export interface Instrument {
  symbol: string;
  min_units: number;
  step: number;
}

export async function fetchInstruments(): Promise<Instrument[]> {
  return parse<Instrument[]>(
    await fetch('/api/v1/instruments', { cache: 'no-store' })
  );
}

export interface NewDeployment {
  name: string;
  strategy_id: string;
  symbol: string;
  timeframe: string;
  units: number;
}

/** Creates a deployment on the paper account. It starts disabled. */
export async function createDeployment(d: NewDeployment): Promise<void> {
  await parse(
    await fetch('/api/v1/deployments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(d),
    })
  );
}

/** Fails while the deployment holds a position. */
export async function deleteDeployment(id: string): Promise<void> {
  await parse(await fetch(`/api/v1/deployments/${id}`, { method: 'DELETE' }));
}

/** True when any kill switch is on. */
export function killActive(switches: KillSwitch[]): boolean {
  return switches.some(k => k.active);
}

export const SCOPE_LABELS: Record<string, string> = {
  global: '全体',
  paper: 'Paper',
  gmo: 'GMOコイン',
};

export function formatYenSigned(v: number): string {
  const sign = v > 0 ? '+' : v < 0 ? '-' : '';
  return `${sign}¥${Math.abs(v).toLocaleString('ja-JP', { maximumFractionDigits: 0 })}`;
}

/** Fired on window after a control (kill switch) changes, so views refresh at once. */
export const BOT_STATUS_CHANGED = 'bot-status-changed';
