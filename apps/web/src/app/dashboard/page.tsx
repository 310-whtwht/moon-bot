'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { Activity, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  BOT_STATUS_CHANGED,
  type BotStatus,
  type Position,
  SCOPE_LABELS,
  fetchBotStatus,
  formatYenSigned,
  killActive,
  setDeploymentEnabled,
} from '@/lib/bot';
import { TIMEFRAMES } from '@/lib/backtest';

const REFRESH_MS = 15000;

const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('ja-JP', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

const sideLabel = (side: Position['side']) =>
  side === 'buy' ? '買い' : '売り';

const pnlColor = (v: number) =>
  v > 0 ? 'text-green-600' : v < 0 ? 'text-red-600' : '';

function Stat({
  label,
  value,
  note,
  tone,
}: {
  label: string;
  value: string;
  note?: string;
  tone?: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className={`text-2xl ${tone ?? ''}`}>{value}</CardTitle>
      </CardHeader>
      {note && (
        <CardContent className="text-xs text-muted-foreground">
          {note}
        </CardContent>
      )}
    </Card>
  );
}

export default function DashboardPage() {
  const [status, setStatus] = useState<BotStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setStatus(await fetchBotStatus());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取得できませんでした');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, REFRESH_MS);
    window.addEventListener(BOT_STATUS_CHANGED, refresh);
    return () => {
      clearInterval(timer);
      window.removeEventListener(BOT_STATUS_CHANGED, refresh);
    };
  }, [refresh]);

  const toggleDeployment = async (id: string, enabled: boolean) => {
    setPending(id);
    try {
      await setDeploymentEnabled(id, enabled);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : '更新できませんでした');
    } finally {
      setPending(null);
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          読み込み中...
        </div>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="container mx-auto p-6">
        <div className="p-4 bg-red-50 border border-red-200 rounded-md text-red-700">
          ダッシュボードのデータを取得できませんでした: {error}
        </div>
      </div>
    );
  }

  const killed = killActive(status.kill_switches);
  const lastSeen = status.heartbeats[0]?.last_seen_at;
  const enabledCount = status.deployments.filter(d => d.enabled).length;
  const deploymentName = (id: string | null) =>
    status.deployments.find(d => d.id === id)?.name ?? '—';
  const timeframeLabel = (tf: string) =>
    TIMEFRAMES.find(t => t.value === tf)?.label ?? tf;

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold">ダッシュボード</h1>
          <p className="text-muted-foreground mt-1">
            自動売買の状態（15秒ごとに更新）
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={refresh}>
          <RefreshCw className="w-4 h-4 mr-2" />
          更新
        </Button>
      </div>

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          {error}
        </div>
      )}

      {killed && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          Kill Switch により新規の発注を停止しています（
          {status.kill_switches
            .filter(k => k.active)
            .map(k => SCOPE_LABELS[k.scope] ?? k.scope)
            .join('・')}
          ）。決済と損切りは動作します。解除は画面上部のボタンから行えます。
        </div>
      )}

      <div className="grid gap-4 grid-cols-2 lg:grid-cols-4">
        <Stat
          label="Bot"
          value={status.bot_alive ? '稼働中' : '停止'}
          tone={status.bot_alive ? 'text-green-600' : 'text-red-600'}
          note={
            lastSeen
              ? `最終確認 ${dateTime(lastSeen)}`
              : 'まだ一度も起動していません'
          }
        />
        <Stat
          label="当日の確定損益"
          value={formatYenSigned(status.daily.pnl)}
          tone={pnlColor(status.daily.pnl)}
          note={`決済 ${status.daily.closed} 件（${dateTime(status.daily.since)} 〜）`}
        />
        <Stat
          label="今週の確定損益"
          value={formatYenSigned(status.weekly.pnl)}
          tone={pnlColor(status.weekly.pnl)}
          note={`決済 ${status.weekly.closed} 件（${dateTime(status.weekly.since)} 〜）`}
        />
        <Stat
          label="建玉 / 稼働中の割り当て"
          value={`${status.open_positions.length} / ${enabledCount}`}
          note={`割り当て ${status.deployments.length} 件`}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>割り当て</CardTitle>
          <CardDescription>
            どの戦略を、どの口座・銘柄で動かすか。無効にしても保有中の建玉は自動では閉じません（決済と損切りは続きます）。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {status.deployments.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              割り当てがありません。
            </p>
          ) : (
            <div className="space-y-3">
              {status.deployments.map(d => (
                <div
                  key={d.id}
                  className="flex items-center justify-between gap-4 p-4 border rounded-lg"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium">{d.name}</span>
                      <Badge variant="secondary">
                        {SCOPE_LABELS[d.broker] ?? d.broker}
                      </Badge>
                      {d.enabled ? (
                        <Badge>有効</Badge>
                      ) : (
                        <Badge variant="outline">無効</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground mt-1">
                      {d.symbol} / {timeframeLabel(d.timeframe)} /{' '}
                      {d.units.toLocaleString('ja-JP')} 通貨 — 戦略:{' '}
                      <Link
                        href={`/strategies/${d.strategy_id}`}
                        className="underline"
                      >
                        {d.strategy_name}
                      </Link>
                      {d.active_version
                        ? `（バージョン ${d.active_version}）`
                        : '（有効なバージョンなし）'}
                    </p>
                  </div>
                  <Switch
                    checked={d.enabled}
                    disabled={pending === d.id}
                    onCheckedChange={(checked: boolean) =>
                      toggleDeployment(d.id, checked)
                    }
                    aria-label={`${d.name} を有効にする`}
                  />
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>保有中の建玉</CardTitle>
          </CardHeader>
          <CardContent>
            {status.open_positions.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                <Activity className="inline w-4 h-4 mr-1" />
                建玉はありません。
              </p>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-muted-foreground border-b">
                    <th className="py-2 pr-3">銘柄</th>
                    <th className="py-2 pr-3">売買</th>
                    <th className="py-2 pr-3 text-right">数量</th>
                    <th className="py-2 pr-3 text-right">建値</th>
                    <th className="py-2 pr-3 text-right">損切り</th>
                    <th className="py-2">建てた時刻</th>
                  </tr>
                </thead>
                <tbody>
                  {status.open_positions.map(p => (
                    <tr key={p.id} className="border-b last:border-0">
                      <td className="py-2 pr-3">
                        {p.symbol}
                        <div className="text-xs text-muted-foreground">
                          {deploymentName(p.deployment_id)}
                        </div>
                      </td>
                      <td className="py-2 pr-3">{sideLabel(p.side)}</td>
                      <td className="py-2 pr-3 text-right">
                        {p.quantity.toLocaleString('ja-JP')}
                      </td>
                      <td className="py-2 pr-3 text-right">
                        {p.open_price.toFixed(3)}
                      </td>
                      <td className="py-2 pr-3 text-right">
                        {p.stop_price != null ? p.stop_price.toFixed(3) : '—'}
                      </td>
                      <td className="py-2 whitespace-nowrap">
                        {dateTime(p.opened_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>最近の決済</CardTitle>
            <CardDescription>損益は手数料を引いた後の金額</CardDescription>
          </CardHeader>
          <CardContent>
            {status.closed_positions.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                まだ決済はありません。
              </p>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-muted-foreground border-b">
                    <th className="py-2 pr-3">銘柄</th>
                    <th className="py-2 pr-3">売買</th>
                    <th className="py-2 pr-3 text-right">建値 → 決済</th>
                    <th className="py-2 pr-3 text-right">損益</th>
                    <th className="py-2">決済時刻</th>
                  </tr>
                </thead>
                <tbody>
                  {status.closed_positions.map(p => (
                    <tr key={p.id} className="border-b last:border-0">
                      <td className="py-2 pr-3">{p.symbol}</td>
                      <td className="py-2 pr-3">{sideLabel(p.side)}</td>
                      <td className="py-2 pr-3 text-right whitespace-nowrap">
                        {p.open_price.toFixed(3)} →{' '}
                        {p.close_price != null ? p.close_price.toFixed(3) : '—'}
                      </td>
                      <td
                        className={`py-2 pr-3 text-right font-medium ${pnlColor(p.realized_pnl ?? 0)}`}
                      >
                        {formatYenSigned(p.realized_pnl ?? 0)}
                      </td>
                      <td className="py-2 whitespace-nowrap">
                        {p.closed_at ? dateTime(p.closed_at) : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
