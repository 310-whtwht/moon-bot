'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ArrowLeft, Trash2 } from 'lucide-react';
import Link from 'next/link';
import { EquityChart } from '@/components/backtest/EquityChart';
import {
  type Backtest,
  EXIT_REASON_LABELS,
  STATUS_LABELS,
  TIMEFRAMES,
  formatNumber,
  formatParams,
  formatPercent,
  formatYen,
  parseResult,
  parseSpec,
} from '@/lib/backtest';

interface Strategy {
  id: string;
  name: string;
}

const POLL_INTERVAL_MS = 2000;

const statusColor = (status: string) => {
  switch (status) {
    case 'completed':
      return 'bg-green-100 text-green-800';
    case 'running':
      return 'bg-blue-100 text-blue-800';
    case 'failed':
      return 'bg-red-100 text-red-800';
    case 'cancelled':
      return 'bg-gray-100 text-gray-800';
    default:
      return 'bg-yellow-100 text-yellow-800';
  }
};

function Metric({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: 'good' | 'bad';
}) {
  const color =
    tone === 'good' ? 'text-green-600' : tone === 'bad' ? 'text-red-600' : '';
  return (
    <div className="p-4 border rounded-lg">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={`text-xl font-bold mt-1 ${color}`}>{value}</p>
    </div>
  );
}

export default function BacktestDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [backtest, setBacktest] = useState<Backtest | null>(null);
  const [strategy, setStrategy] = useState<Strategy | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const backtestId = params.id as string;

  const fetchBacktest = useCallback(async () => {
    try {
      const response = await fetch(`/api/v1/backtests/${backtestId}`);
      if (!response.ok) {
        throw new Error('バックテストを取得できませんでした');
      }
      const data = await response.json();
      setBacktest(data.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setLoading(false);
    }
  }, [backtestId]);

  useEffect(() => {
    if (backtestId) {
      fetchBacktest();
    }
  }, [backtestId, fetchBacktest]);

  // Poll while the bot is working on it.
  const inProgress =
    backtest?.status === 'pending' || backtest?.status === 'running';
  useEffect(() => {
    if (!inProgress) {
      return;
    }
    const timer = setInterval(fetchBacktest, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [inProgress, fetchBacktest]);

  const strategyId = backtest?.strategy_id;
  useEffect(() => {
    if (!strategyId) {
      return;
    }
    fetch(`/api/v1/strategies/${strategyId}`)
      .then(r => (r.ok ? r.json() : null))
      .then(d => d && setStrategy(d.data))
      .catch(() => undefined);
  }, [strategyId]);

  const handleDelete = async () => {
    if (!window.confirm('このバックテストを削除しますか？元に戻せません。')) {
      return;
    }
    const response = await fetch(`/api/v1/backtests/${backtestId}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      setError('削除できませんでした');
      return;
    }
    router.push('/backtests');
  };

  const handleCancel = async () => {
    const response = await fetch(`/api/v1/backtests/${backtestId}/cancel`, {
      method: 'POST',
    });
    if (!response.ok) {
      setError('キャンセルできませんでした');
      return;
    }
    await fetchBacktest();
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

  if (error || !backtest) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64 text-red-500">
          {error || 'バックテストが見つかりません'}
        </div>
      </div>
    );
  }

  const result = parseResult(backtest.results);
  const spec = parseSpec(backtest.parameters);
  const m = result?.metrics;
  const timeframeLabel =
    TIMEFRAMES.find(t => t.value === spec?.timeframe)?.label ?? spec?.timeframe;

  return (
    <div className="container mx-auto p-6">
      <div className="mb-6">
        <Link
          href="/backtests"
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          バックテスト一覧へ
        </Link>
      </div>

      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-3xl font-bold">{backtest.name}</h1>
          <p className="text-muted-foreground mt-2">
            {strategy?.name && `戦略: ${strategy.name}`}
          </p>
        </div>
        <div className="flex gap-2">
          {inProgress && (
            <Button variant="outline" onClick={handleCancel}>
              キャンセル
            </Button>
          )}
          <Button
            variant="outline"
            onClick={handleDelete}
            disabled={backtest.status === 'running'}
            className="text-red-600 hover:text-red-700"
          >
            <Trash2 className="w-4 h-4 mr-2" />
            削除
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle>条件</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <div>
                <p className="text-muted-foreground">状態</p>
                <Badge className={`mt-1 ${statusColor(backtest.status)}`}>
                  {STATUS_LABELS[backtest.status] ?? backtest.status}
                </Badge>
                {inProgress && (
                  <p className="text-xs text-muted-foreground mt-1">
                    bot が処理しています（自動で更新します）
                  </p>
                )}
              </div>
              <div>
                <p className="text-muted-foreground">銘柄・時間足</p>
                <p className="mt-1">
                  {backtest.symbols.join(', ')} {timeframeLabel}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground">期間</p>
                <p className="mt-1">
                  {new Date(backtest.start_date).toLocaleDateString('ja-JP')} 〜{' '}
                  {new Date(backtest.end_date).toLocaleDateString('ja-JP')}
                </p>
              </div>
              {spec && (
                <>
                  <div>
                    <p className="text-muted-foreground">数量・初期資金</p>
                    <p className="mt-1">
                      {spec.units.toLocaleString('ja-JP')} 通貨 /{' '}
                      {formatYen(spec.initial_balance)}
                    </p>
                  </div>
                  <div>
                    <p className="text-muted-foreground">戦略パラメータ</p>
                    <p className="mt-1 font-mono text-xs break-words">
                      {spec.strategy_type} {formatParams(spec.strategy_params)}
                    </p>
                  </div>
                </>
              )}
              {backtest.error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700">
                  {backtest.error}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="lg:col-span-2">
          {!result || !m ? (
            <Card>
              <CardContent className="py-12 text-center text-muted-foreground">
                {inProgress
                  ? '実行中です。完了すると結果が表示されます。'
                  : '表示できる結果がありません。'}
              </CardContent>
            </Card>
          ) : (
            <Tabs defaultValue="summary" className="w-full">
              <TabsList>
                <TabsTrigger value="summary">概要</TabsTrigger>
                <TabsTrigger value="trades">
                  取引一覧（{result.trades.length}）
                </TabsTrigger>
              </TabsList>

              <TabsContent value="summary" className="mt-6 space-y-6">
                <div className="grid gap-4 grid-cols-2 md:grid-cols-4">
                  <Metric
                    label="損益"
                    value={formatYen(m.net_profit)}
                    tone={m.net_profit >= 0 ? 'good' : 'bad'}
                  />
                  <Metric
                    label="リターン"
                    value={formatPercent(m.total_return, true)}
                    tone={m.total_return >= 0 ? 'good' : 'bad'}
                  />
                  <Metric
                    label="最大ドローダウン"
                    value={formatPercent(m.max_drawdown)}
                  />
                  <Metric
                    label="プロフィットファクター"
                    value={
                      m.profit_factor ? formatNumber(m.profit_factor) : '—'
                    }
                  />
                  <Metric label="取引数" value={String(m.num_trades)} />
                  <Metric label="勝率" value={formatPercent(m.win_rate)} />
                  <Metric
                    label="シャープレシオ"
                    value={formatNumber(m.sharpe)}
                  />
                  <Metric label="手数料合計" value={formatYen(m.total_fees)} />
                </div>

                <Card>
                  <CardHeader>
                    <CardTitle>損益曲線</CardTitle>
                    <CardDescription>
                      点線は初期資金 {formatYen(m.initial_balance)}。CAGR{' '}
                      {formatPercent(m.cagr, true)}、在場率{' '}
                      {formatPercent(m.exposure)}
                      {result.skipped_entries > 0 &&
                        `、証拠金不足で見送り ${result.skipped_entries} 回`}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <EquityChart
                      points={result.equity}
                      initialBalance={m.initial_balance}
                    />
                  </CardContent>
                </Card>

                <p className="text-xs text-muted-foreground">
                  約定モデル: 確定足のシグナルを次の足の始値で執行、買いは
                  ASK・売りは BID、損切りは足の中で判定、API 手数料
                  0.002%。スリッページとスワップは含みません。
                  過去の成績は将来の成果を保証しません。
                </p>
              </TabsContent>

              <TabsContent value="trades" className="mt-6">
                <Card>
                  <CardContent className="pt-6 overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-left text-muted-foreground border-b">
                          <th className="py-2 pr-4">売買</th>
                          <th className="py-2 pr-4">エントリー</th>
                          <th className="py-2 pr-4">決済</th>
                          <th className="py-2 pr-4 text-right">損益</th>
                          <th className="py-2">決済理由</th>
                        </tr>
                      </thead>
                      <tbody>
                        {result.trades.map((t, i) => (
                          <tr key={i} className="border-b last:border-0">
                            <td className="py-2 pr-4">
                              <Badge
                                variant={
                                  t.side === 'LONG' ? 'default' : 'secondary'
                                }
                              >
                                {t.side === 'LONG' ? '買い' : '売り'}
                              </Badge>
                            </td>
                            <td className="py-2 pr-4 whitespace-nowrap">
                              {new Date(t.entry_time).toLocaleString('ja-JP')}
                              <span className="text-muted-foreground ml-2">
                                {t.entry_price.toFixed(3)}
                              </span>
                            </td>
                            <td className="py-2 pr-4 whitespace-nowrap">
                              {new Date(t.exit_time).toLocaleString('ja-JP')}
                              <span className="text-muted-foreground ml-2">
                                {t.exit_price.toFixed(3)}
                              </span>
                            </td>
                            <td
                              className={`py-2 pr-4 text-right font-medium ${t.pnl >= 0 ? 'text-green-600' : 'text-red-600'}`}
                            >
                              {formatYen(t.pnl)}
                            </td>
                            <td className="py-2">
                              {EXIT_REASON_LABELS[t.exit_reason] ??
                                t.exit_reason}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </CardContent>
                </Card>
              </TabsContent>
            </Tabs>
          )}
        </div>
      </div>
    </div>
  );
}
