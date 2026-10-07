'use client';

import { Spinner } from '@/components/ui/spinner';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  type ChartLevel,
  type ChartLine,
  type ChartMarker,
  LiveChart,
  SIGNAL_COLOR,
} from '@/components/chart/LiveChart';
import {
  type Deployment,
  type Position,
  fetchBotStatus,
  formatYenSigned,
} from '@/lib/bot';
import {
  type BarDecision,
  type ChartData,
  describeResult,
  TIMEFRAME_SECONDS,
  ema,
  fetchChart,
  withQuote,
} from '@/lib/chart';
import { useGmoTicker } from '@/lib/useGmoTicker';

const REFRESH_MS = 10000;
const TIMEFRAMES = [
  { value: '1m', label: '1分' },
  { value: '5m', label: '5分' },
  { value: '15m', label: '15分' },
  { value: '30m', label: '30分' },
  { value: '1h', label: '1時間' },
];

const SIGNAL_LABELS = { buy: '買', sell: '売', exit: '手仕舞', stop: '損切' };

const ACTION_LABELS: Record<BarDecision['action'], string> = {
  HOLD: 'シグナルなし',
  ENTER_LONG: '買いシグナル',
  ENTER_SHORT: '売りシグナル',
  EXIT: '決済シグナル',
};
const HOLDING_LABELS: Record<BarDecision['holding'], string> = {
  '': 'なし',
  BUY: '買い保有中',
  SELL: '売り保有中',
};
const DETAIL_LABELS: Record<string, string> = {
  fast: '短期',
  slow: '長期',
  diff: '差',
  atr: 'ATR',
};

/** "fast=1 slow=2" -> "短期 1 / 長期 2"; anything else is shown as it is. */
function describeDetail(detail: string): string {
  if (detail === 'warming up') {
    return '指標の準備中';
  }
  const parts = detail.split(' ').map(part => part.split('='));
  if (!detail || parts.some(part => part.length !== 2)) {
    return detail || '—';
  }
  return parts
    .map(([name, value]) => `${DETAIL_LABELS[name!] ?? name} ${value}`)
    .join(' / ');
}

const hourMinute = (ms: number) =>
  new Date(ms).toLocaleTimeString('ja-JP', {
    hour: '2-digit',
    minute: '2-digit',
  });
/** The span of a bar in local time, e.g. "10/5 11:00〜12:00". */
function barSpan(iso: string, seconds: number): string {
  const start = Date.parse(iso);
  const day = new Date(start).toLocaleDateString('ja-JP', {
    month: 'numeric',
    day: 'numeric',
  });
  return `${day} ${hourMinute(start)}〜${hourMinute(start + seconds * 1000)}`;
}

const price = (v: number) => v.toFixed(3);
const signed = (v: number, digits = 3) =>
  `${v > 0 ? '+' : ''}${v.toFixed(digits)}`;
const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('ja-JP', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
const sideLabel = (side: Position['side']) =>
  side === 'buy' ? '買い' : '売り';
const pnlColor = (v: number) =>
  v > 0 ? 'text-green-600' : v < 0 ? 'text-red-600' : '';
const unix = (iso: string) => Math.floor(Date.parse(iso) / 1000);

/** A position opening or closing, announced once when it is first seen. */
function describeFill(p: Position): string {
  if (p.status === 'open') {
    return `新規約定: ${sideLabel(p.side)} ${p.quantity.toLocaleString()} 通貨 @ ${price(p.open_price)}（${dateTime(p.opened_at)}）`;
  }
  return `決済: ${sideLabel(p.side)}の建玉を ${price(p.close_price ?? 0)} で決済、損益 ${formatYenSigned(p.realized_pnl ?? 0)}（${dateTime(p.closed_at ?? p.opened_at)}）`;
}

function Stat({
  label,
  value,
  note,
  tone,
}: {
  label: string;
  value: string;
  note?: string | undefined;
  tone?: string;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{label}</CardDescription>
        <CardTitle className={`text-2xl tabular-nums ${tone ?? ''}`}>
          {value}
        </CardTitle>
      </CardHeader>
      {note && (
        <CardContent className="text-xs text-muted-foreground">
          {note}
        </CardContent>
      )}
    </Card>
  );
}

export default function ChartPage() {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [symbol, setSymbol] = useState('USD_JPY');
  const [timeframe, setTimeframe] = useState('1h');
  const [chosen, setChosen] = useState(false);
  const [data, setData] = useState<{ chart: ChartData; seq: number } | null>(
    null
  );
  const [error, setError] = useState<string | null>(null);
  const [fills, setFills] = useState<string[]>([]);
  const [showSignals, setShowSignals] = useState(true);
  const [nowMs, setNowMs] = useState(() => Date.now());
  const seen = useRef<Set<string> | null>(null);
  const { quote, state } = useGmoTicker(symbol);

  // Start on what the bot is trading.
  useEffect(() => {
    fetchBotStatus()
      .then(status => {
        setDeployments(status.deployments);
        const first =
          status.deployments.find(d => d.enabled) ?? status.deployments[0];
        if (first) {
          setSymbol(first.symbol);
          setTimeframe(first.timeframe);
        }
      })
      .catch(() => undefined)
      .finally(() => setChosen(true));
  }, []);

  useEffect(() => {
    if (!chosen) {
      return;
    }
    let cancelled = false;
    let seq = 0;
    seen.current = null;
    setData(null);

    const load = async () => {
      try {
        const chart = await fetchChart(symbol, timeframe);
        if (cancelled) {
          return;
        }
        // Announce positions that opened or closed since the last look.
        const keys = chart.positions.map(p => `${p.id}:${p.status}`);
        if (seen.current) {
          const fresh = chart.positions.filter(
            p => !seen.current!.has(`${p.id}:${p.status}`)
          );
          if (fresh.length > 0) {
            setFills(prev => [...fresh.map(describeFill), ...prev].slice(0, 5));
          }
        }
        seen.current = new Set(keys);
        seq += 1;
        setData({ chart, seq });
        setError(null);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '取得できませんでした');
        }
      }
    };
    load();
    const timer = setInterval(load, REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [chosen, symbol, timeframe]);

  useEffect(() => {
    const timer = setInterval(() => setNowMs(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  const chart = data?.chart ?? null;
  const barSeconds = TIMEFRAME_SECONDS[timeframe] ?? 3600;
  const strategy = chart?.strategy ?? null;
  const emaCross = strategy?.type === 'ema_cross' ? strategy.params : null;
  const fastPeriod = emaCross?.fast_period;
  const slowPeriod = emaCross?.slow_period;

  const bars = useMemo(
    () => withQuote(chart?.bars ?? [], quote, timeframe),
    [chart, quote, timeframe]
  );

  const { lines, fast, slow } = useMemo(() => {
    if (!fastPeriod || !slowPeriod) {
      return { lines: [] as ChartLine[], fast: [], slow: [] };
    }
    const closes = bars.map(b => b.close);
    const fast = ema(closes, fastPeriod);
    const slow = ema(closes, slowPeriod);
    const lines: ChartLine[] = [
      { label: `EMA ${fastPeriod}`, color: '#2563eb', values: fast },
      { label: `EMA ${slowPeriod}`, color: '#f59e0b', values: slow },
    ];
    return { lines, fast, slow };
  }, [bars, fastPeriod, slowPeriod]);

  const positions = chart?.positions;
  const signals = chart?.signals;
  const markers = useMemo<ChartMarker[]>(() => {
    const fills = (positions ?? []).flatMap(p => {
      const entry: ChartMarker = {
        time: unix(p.opened_at),
        kind: p.side,
        text: `${sideLabel(p.side)} ${price(p.open_price)}`,
      };
      if (p.status !== 'closed' || !p.closed_at) {
        return [entry];
      }
      return [
        entry,
        {
          time: unix(p.closed_at),
          kind: 'exit' as const,
          text: `決済 ${formatYenSigned(p.realized_pnl ?? 0)}`,
        },
      ];
    });
    const replayed = (showSignals ? (signals ?? []) : []).map(
      (signal): ChartMarker => ({
        time: signal.time,
        kind:
          signal.kind === 'buy' || signal.kind === 'sell'
            ? signal.kind
            : 'exit',
        text: SIGNAL_LABELS[signal.kind],
        hypothetical: true,
      })
    );
    return [...replayed, ...fills];
  }, [positions, signals, showSignals]);
  const open = useMemo(
    () => (positions ?? []).filter(p => p.status === 'open'),
    [positions]
  );
  const levels = useMemo<ChartLevel[]>(
    () =>
      open.flatMap(p => [
        { price: p.open_price, label: '建値', color: '#2563eb' },
        ...(p.stop_price != null
          ? [
              {
                price: p.stop_price,
                label: '損切り',
                color: '#dc2626',
                dashed: true,
              },
            ]
          : []),
      ]),
    [open]
  );

  // The difference the strategy watches: on the last closed bar, and right now.
  const last = bars.length - 1;
  const diffAt = (i: number) => {
    const f = fast[i];
    const s = slow[i];
    return f != null && s != null ? f - s : null;
  };
  const lastBar = bars[last];
  const forming = lastBar != null && lastBar.time + barSeconds > nowMs / 1000;
  const closedDiff = diffAt(forming ? last - 1 : last);
  const liveDiff = diffAt(last);
  const secondsLeft =
    forming && lastBar
      ? Math.max(0, Math.round(lastBar.time + barSeconds - nowMs / 1000))
      : null;

  // Unrealised P&L in yen is only meaningful for pairs quoted in JPY.
  const unrealised =
    quote && symbol.endsWith('_JPY') && open.length > 0
      ? open.reduce(
          (sum, p) =>
            sum +
            (p.side === 'buy'
              ? quote.bid - p.open_price
              : p.open_price - quote.ask) *
              p.quantity,
          0
        )
      : null;

  const symbols = Array.from(
    new Set([symbol, ...deployments.map(d => d.symbol)])
  );
  const traded = deployments.find(d => d.symbol === symbol);
  const timeframes = TIMEFRAMES.some(t => t.value === timeframe)
    ? TIMEFRAMES
    : [...TIMEFRAMES, { value: timeframe, label: timeframe }];
  const decisions = chart?.decisions ?? [];
  const recent = [...(positions ?? [])].reverse().slice(0, 20);

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex flex-wrap justify-between items-start gap-4">
        <div>
          <h1 className="text-3xl font-bold">チャート</h1>
          <p className="text-muted-foreground mt-1">
            GMOコインの実レート（BID）に、bot の約定を重ねて表示します
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {symbols.length > 1 &&
            symbols.map(s => (
              <Button
                key={s}
                size="sm"
                variant={s === symbol ? 'default' : 'outline'}
                onClick={() => {
                  setSymbol(s);
                  // Jump to the timeframe that symbol is traded on.
                  const traded = deployments.find(d => d.symbol === s);
                  if (traded) {
                    setTimeframe(traded.timeframe);
                  }
                }}
              >
                {s.replace('_', '/')}
              </Button>
            ))}
          {timeframes.map(t => (
            <Button
              key={t.value}
              size="sm"
              variant={t.value === timeframe ? 'default' : 'outline'}
              onClick={() => setTimeframe(t.value)}
              title={
                traded?.timeframe === t.value ? 'bot が売買している足' : ''
              }
            >
              {t.label}
              {traded?.timeframe === t.value && ' ●'}
            </Button>
          ))}
        </div>
      </div>

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          チャートのデータを取得できませんでした: {error}
        </div>
      )}

      {fills.length > 0 && (
        <div className="p-3 bg-blue-50 border border-blue-200 rounded-md text-blue-800 text-sm space-y-1">
          {fills.map((fill, i) => (
            <div key={`${fill}-${i}`} className="flex justify-between gap-4">
              <span>{fill}</span>
              {i === 0 && (
                <button className="underline" onClick={() => setFills([])}>
                  閉じる
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      <div className="grid gap-4 grid-cols-2 lg:grid-cols-4">
        <Stat
          label={`${symbol.replace('_', '/')} BID / ASK`}
          value={quote ? `${price(quote.bid)} / ${price(quote.ask)}` : '—'}
          note={
            state === 'live'
              ? 'リアルタイムで受信中'
              : state === 'connecting'
                ? '接続中...'
                : '配信停止中（市場の休場、または再接続中）'
          }
          tone={state === 'live' ? '' : 'text-muted-foreground'}
        />
        <Stat
          label="短期 EMA − 長期 EMA"
          value={liveDiff != null ? signed(liveDiff) : '—'}
          tone={liveDiff != null ? pnlColor(liveDiff) : ''}
          note={
            emaCross
              ? `確定足では ${closedDiff != null ? signed(closedDiff) : '—'}。足の確定時に符号が変わると売買します`
              : 'この足で売買している戦略はありません'
          }
        />
        <Stat
          label="次の足の確定まで"
          value={
            secondsLeft != null
              ? `${Math.floor(secondsLeft / 60)}:${String(secondsLeft % 60).padStart(2, '0')}`
              : '—'
          }
          note={
            strategy
              ? `${strategy.type} ${strategy.version}（${strategy.enabled ? '稼働中' : '停止中'}）`
              : undefined
          }
        />
        <Stat
          label="建玉 / 含み損益"
          value={
            open.length === 0
              ? 'なし'
              : `${open.length} 件 / ${unrealised != null ? formatYenSigned(unrealised) : '—'}`
          }
          tone={unrealised != null ? pnlColor(unrealised) : ''}
          note={
            open[0]
              ? `${sideLabel(open[0].side)} ${price(open[0].open_price)}、損切り ${open[0].stop_price != null ? price(open[0].stop_price) : '—'}`
              : 'シグナル待ち'
          }
        />
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base flex flex-wrap items-center gap-2">
            {symbol.replace('_', '/')}{' '}
            {timeframes.find(t => t.value === timeframe)?.label}足
            {lines.map(line => (
              <Badge
                key={line.label}
                variant="outline"
                style={{ color: line.color, borderColor: line.color }}
              >
                {line.label}
              </Badge>
            ))}
          </CardTitle>
          <CardDescription>
            時刻は日本時間。▲ 買い・▼ 売り・●
            決済（実際の約定）。青線は建値、赤の破線は損切り
          </CardDescription>
          {signals && (
            <div className="flex flex-wrap items-center gap-3 pt-1 text-sm">
              <Button
                size="sm"
                variant={showSignals ? 'default' : 'outline'}
                onClick={() => setShowSignals(v => !v)}
              >
                過去のシグナル {showSignals ? '表示中' : '非表示'}
              </Button>
              <span className="text-muted-foreground">
                <span style={{ color: SIGNAL_COLOR }}>紫の印</span>
                は「今の設定で動いていたら売買していた位置」の再現です（
                {signals.length}{' '}
                件）。実際の約定ではなく、スプレッドと手数料は含みません
              </span>
            </div>
          )}
        </CardHeader>
        <CardContent>
          {chart ? (
            <LiveChart
              historyKey={`${symbol}:${timeframe}|${data?.seq}`}
              bars={bars}
              barSeconds={barSeconds}
              lines={lines}
              markers={markers}
              levels={levels}
            />
          ) : (
            <div className="flex items-center justify-center h-[480px] text-muted-foreground">
              {error ? (
                '表示できません'
              ) : (
                <Spinner label="読み込み中...（初回は数秒かかります）" />
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {strategy && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">足ごとの判定</CardTitle>
            <CardDescription>
              bot
              が確定足を見て出した判定の記録（新しい順、最大48件）。「シグナルなし」は売買の条件を満たさなかった足で、正常な待機です。シグナルが出たのに発注しなかった場合は、「結果」に理由が出ます
            </CardDescription>
          </CardHeader>
          <CardContent>
            {decisions.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                まだ記録はありません。次の足が確定すると1行増えます
              </p>
            ) : (
              <div className="overflow-x-auto max-h-96 overflow-y-auto">
                <table className="w-full text-sm">
                  <thead className="text-left text-muted-foreground">
                    <tr>
                      <th className="py-2 pr-4">足</th>
                      <th className="py-2 pr-4">終値</th>
                      <th className="py-2 pr-4">判定</th>
                      <th className="py-2 pr-4">建玉</th>
                      <th className="py-2 pr-4">結果</th>
                      <th className="py-2">戦略が見た値</th>
                    </tr>
                  </thead>
                  <tbody>
                    {decisions.map(d => (
                      <tr key={d.bar_time} className="border-t tabular-nums">
                        <td className="py-2 pr-4 whitespace-nowrap">
                          {barSpan(d.bar_time, barSeconds)}
                        </td>
                        <td className="py-2 pr-4">{price(d.close)}</td>
                        <td
                          className={`py-2 pr-4 whitespace-nowrap ${d.action === 'HOLD' ? 'text-muted-foreground' : 'font-semibold'}`}
                        >
                          {ACTION_LABELS[d.action] ?? d.action}
                        </td>
                        <td className="py-2 pr-4 whitespace-nowrap">
                          {HOLDING_LABELS[d.holding] ?? d.holding}
                        </td>
                        <td className="py-2 pr-4">
                          {describeResult(d.result).map(line => (
                            <div
                              key={line}
                              className={
                                line.startsWith('発注せず') ||
                                line.startsWith('エラー')
                                  ? 'text-yellow-700 font-medium'
                                  : ''
                              }
                            >
                              {line}
                            </div>
                          ))}
                          {!d.result && (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </td>
                        <td className="py-2 text-muted-foreground">
                          {describeDetail(d.detail)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">この期間の約定</CardTitle>
          <CardDescription>
            チャートに表示している範囲の建玉（新しい順、最大20件）
          </CardDescription>
        </CardHeader>
        <CardContent>
          {recent.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              まだ約定はありません
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-left text-muted-foreground">
                  <tr>
                    <th className="py-2 pr-4">売買</th>
                    <th className="py-2 pr-4">数量</th>
                    <th className="py-2 pr-4">新規</th>
                    <th className="py-2 pr-4">建値</th>
                    <th className="py-2 pr-4">損切り</th>
                    <th className="py-2 pr-4">決済</th>
                    <th className="py-2 pr-4">決済値</th>
                    <th className="py-2 text-right">損益</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.map(p => (
                    <tr key={p.id} className="border-t tabular-nums">
                      <td className="py-2 pr-4">{sideLabel(p.side)}</td>
                      <td className="py-2 pr-4">
                        {p.quantity.toLocaleString()}
                      </td>
                      <td className="py-2 pr-4">{dateTime(p.opened_at)}</td>
                      <td className="py-2 pr-4">{price(p.open_price)}</td>
                      <td className="py-2 pr-4">
                        {p.stop_price != null ? price(p.stop_price) : '—'}
                      </td>
                      <td className="py-2 pr-4">
                        {p.closed_at ? dateTime(p.closed_at) : '保有中'}
                      </td>
                      <td className="py-2 pr-4">
                        {p.close_price != null ? price(p.close_price) : '—'}
                      </td>
                      <td
                        className={`py-2 text-right ${pnlColor(p.realized_pnl ?? 0)}`}
                      >
                        {p.realized_pnl != null
                          ? formatYenSigned(p.realized_pnl)
                          : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
