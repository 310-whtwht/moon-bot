'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CheckCircle,
  Download,
  Loader2,
  RefreshCw,
  XCircle,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Spinner } from '@/components/ui/spinner';
import {
  DATA_TIMEFRAMES,
  type DataImport,
  type ImportStatus,
  type MarketData,
  cancelImport,
  fetchMarketData,
  requestImport,
  sortCoverage,
  timeframeLabel,
  usable,
} from '@/lib/marketData';

const BUSY_REFRESH_MS = 4000;
const IDLE_REFRESH_MS = 30000;

const selectClass =
  'h-10 w-full rounded-md border border-input bg-background px-3 text-sm';

const pair = (symbol: string) => symbol.replace('_', '/');
const day = (iso: string) =>
  new Date(iso).toLocaleDateString('ja-JP', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
  });
const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('ja-JP', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

const STATUS_LABELS: Record<ImportStatus, string> = {
  pending: '待機中',
  running: '取り込み中',
  completed: '完了',
  failed: '失敗',
  cancelled: '取消',
};

const active = (i: DataImport) =>
  i.status === 'pending' || i.status === 'running';

/** "ASK 2025-06-01" -> "買値（ASK）を 2025/6/1 まで". */
function describeProgress(progress: string): string {
  const [side, date] = progress.split(' ');
  if (!side || !date) {
    return '';
  }
  const label = side === 'BID' ? '売値（BID）' : '買値（ASK）';
  return `${label}を ${day(date)} まで`;
}

export default function MarketDataPage() {
  const [data, setData] = useState<MarketData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [symbol, setSymbol] = useState('');
  const [timeframe, setTimeframe] = useState('1h');
  const [from, setFrom] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setData(await fetchMarketData());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取得できませんでした');
    } finally {
      setLoading(false);
    }
  }, []);

  // Refresh quickly while something is downloading, slowly otherwise.
  const busy = data?.imports.some(active) ?? false;
  useEffect(() => {
    refresh();
    const timer = setInterval(
      refresh,
      busy ? BUSY_REFRESH_MS : IDLE_REFRESH_MS
    );
    return () => clearInterval(timer);
  }, [refresh, busy]);

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <Spinner className="h-64" />
      </div>
    );
  }
  if (!data) {
    return (
      <div className="container mx-auto p-6">
        <div className="p-4 bg-red-50 border border-red-200 rounded-md text-red-700">
          銘柄とデータの情報を取得できませんでした: {error}
        </div>
      </div>
    );
  }

  const chosenSymbol = symbol || data.instruments[0]?.symbol || '';
  const chosenFrom = from || data.history_start;
  const coverage = sortCoverage(data.coverage);
  const existing = coverage.find(
    c => c.symbol === chosenSymbol && c.timeframe === timeframe
  );

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSubmitting(true);
    setFormError(null);
    try {
      await requestImport(chosenSymbol, timeframe, chosenFrom);
      await refresh();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '登録できませんでした');
    } finally {
      setSubmitting(false);
    }
  };

  const cancel = async (id: string) => {
    try {
      await cancelImport(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取り消せませんでした');
    }
    await refresh();
  };

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold">銘柄・データ</h1>
          <p className="text-muted-foreground mt-1">
            バックテストに使う過去の足データを、銘柄と足を選んで取り込みます
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

      <Card>
        <CardHeader>
          <CardTitle>データを取り込む</CardTitle>
          <CardDescription>
            GMOコインから、売値（BID）と買値（ASK）の足を開始日から今日まで取り込みます。1つの足で数分〜十数分かかり、依頼は1つずつ順番に処理されます。すでにあるデータの続きから取り込むので、同じ銘柄・足をもう一度依頼すると、最新まで更新されます
          </CardDescription>
        </CardHeader>
        <CardContent>
          {data.instruments.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              GMOコインから銘柄の一覧を取得できませんでした。少し待ってから「更新」を押してください
            </p>
          ) : (
            <form onSubmit={submit} className="space-y-4">
              <div className="grid gap-4 md:grid-cols-4">
                <div className="space-y-1">
                  <Label htmlFor="import-symbol">銘柄</Label>
                  <select
                    id="import-symbol"
                    className={selectClass}
                    value={chosenSymbol}
                    onChange={e => setSymbol(e.target.value)}
                  >
                    {data.instruments.map(i => (
                      <option key={i.symbol} value={i.symbol}>
                        {pair(i.symbol)}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="import-timeframe">足</Label>
                  <select
                    id="import-timeframe"
                    className={selectClass}
                    value={timeframe}
                    onChange={e => setTimeframe(e.target.value)}
                  >
                    {DATA_TIMEFRAMES.map(t => (
                      <option key={t.value} value={t.value}>
                        {t.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="import-from">開始日</Label>
                  <Input
                    id="import-from"
                    type="date"
                    min={data.history_start}
                    value={chosenFrom}
                    onChange={e => setFrom(e.target.value)}
                    required
                  />
                </div>
                <div className="flex items-end">
                  <Button type="submit" disabled={submitting}>
                    <Download className="w-4 h-4 mr-2" />
                    {submitting ? '登録中...' : '取り込む'}
                  </Button>
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                {existing
                  ? `${pair(chosenSymbol)} の${timeframeLabel(timeframe)}は ${day(existing.first)} 〜 ${day(existing.last)} のデータがあります。取り込むと、その続きから最新までを足します（開始日より前のデータは増えません）。`
                  : `${pair(chosenSymbol)} の${timeframeLabel(timeframe)}は、まだデータがありません。`}{' '}
                選べるのは円建ての通貨ペアです。GMOコインのデータは{' '}
                {day(data.history_start)}{' '}
                からあります。足が短いほど本数が多く、時間がかかります。
              </p>
              {formError && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
                  {formError}
                </div>
              )}
            </form>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>取り込みの状況</CardTitle>
          <CardDescription>
            直近20件。取り込み中は数秒ごとに更新します
          </CardDescription>
        </CardHeader>
        <CardContent>
          {data.imports.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              まだ取り込みの依頼はありません
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-left text-muted-foreground">
                  <tr>
                    <th className="py-2 pr-4">銘柄・足</th>
                    <th className="py-2 pr-4">開始日</th>
                    <th className="py-2 pr-4">状態</th>
                    <th className="py-2 pr-4 text-right">取り込んだ本数</th>
                    <th className="py-2 pr-4">進み具合</th>
                    <th className="py-2 pr-4">依頼</th>
                    <th className="py-2" />
                  </tr>
                </thead>
                <tbody>
                  {data.imports.map(i => (
                    <tr key={i.id} className="border-t tabular-nums">
                      <td className="py-2 pr-4 whitespace-nowrap">
                        {pair(i.symbol)} {timeframeLabel(i.timeframe)}
                      </td>
                      <td className="py-2 pr-4">{day(i.from_date)}</td>
                      <td className="py-2 pr-4 whitespace-nowrap">
                        <span className="inline-flex items-center gap-1">
                          {i.status === 'running' && (
                            <Loader2 className="w-4 h-4 animate-spin" />
                          )}
                          {i.status === 'completed' && (
                            <CheckCircle className="w-4 h-4 text-green-600" />
                          )}
                          {i.status === 'failed' && (
                            <XCircle className="w-4 h-4 text-red-600" />
                          )}
                          {STATUS_LABELS[i.status] ?? i.status}
                        </span>
                      </td>
                      <td className="py-2 pr-4 text-right">
                        {i.bars_stored.toLocaleString('ja-JP')}
                      </td>
                      <td className="py-2 pr-4">
                        {i.status === 'failed' ? (
                          <span className="text-red-600">{i.error}</span>
                        ) : i.status === 'running' ? (
                          describeProgress(i.progress)
                        ) : i.status === 'completed' && i.bars_stored === 0 ? (
                          'すでに最新でした'
                        ) : (
                          ''
                        )}
                      </td>
                      <td className="py-2 pr-4 whitespace-nowrap">
                        {dateTime(i.created_at)}
                      </td>
                      <td className="py-2 text-right">
                        {i.status === 'pending' && (
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => cancel(i.id)}
                          >
                            取り消す
                          </Button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>保存済みのデータ</CardTitle>
          <CardDescription>
            バックテストで選べるのは、売値（BID）と買値（ASK）が揃っている銘柄・足です
          </CardDescription>
        </CardHeader>
        <CardContent>
          {coverage.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              まだデータがありません。上のフォームから取り込んでください
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-left text-muted-foreground">
                  <tr>
                    <th className="py-2 pr-4">銘柄</th>
                    <th className="py-2 pr-4">足</th>
                    <th className="py-2 pr-4">期間</th>
                    <th className="py-2 pr-4 text-right">本数（BID / ASK）</th>
                    <th className="py-2">バックテスト</th>
                  </tr>
                </thead>
                <tbody>
                  {coverage.map(c => (
                    <tr
                      key={`${c.symbol}-${c.timeframe}`}
                      className="border-t tabular-nums"
                    >
                      <td className="py-2 pr-4">{pair(c.symbol)}</td>
                      <td className="py-2 pr-4">
                        {timeframeLabel(c.timeframe)}
                      </td>
                      <td className="py-2 pr-4 whitespace-nowrap">
                        {day(c.first)} 〜 {day(c.last)}
                      </td>
                      <td className="py-2 pr-4 text-right">
                        {c.bid_bars.toLocaleString('ja-JP')} /{' '}
                        {c.ask_bars.toLocaleString('ja-JP')}
                      </td>
                      <td className="py-2">
                        {usable(c) ? (
                          <Badge>使えます</Badge>
                        ) : (
                          <Badge variant="outline">
                            BID と ASK が揃っていません
                          </Badge>
                        )}
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
