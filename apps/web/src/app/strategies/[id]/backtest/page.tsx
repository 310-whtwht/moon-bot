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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ArrowLeft, Play } from 'lucide-react';
import Link from 'next/link';
import { type StrategyVersion, TIMEFRAMES, formatParams } from '@/lib/backtest';

interface Strategy {
  id: string;
  name: string;
}

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm';

export default function BacktestPage() {
  const params = useParams();
  const router = useRouter();
  const strategyId = params.id as string;

  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [strategy, setStrategy] = useState<Strategy | null>(null);
  const [versions, setVersions] = useState<StrategyVersion[]>([]);
  const [form, setForm] = useState({
    name: '',
    strategy_version_id: '',
    symbol: 'USD_JPY',
    timeframe: '1h',
    start_date: '2023-10-28',
    end_date: new Date().toISOString().slice(0, 10),
    units: '100',
    initial_balance: '30000',
  });

  const load = useCallback(async () => {
    try {
      const [s, v] = await Promise.all([
        fetch(`/api/v1/strategies/${strategyId}`),
        fetch(`/api/v1/strategies/${strategyId}/versions`),
      ]);
      if (!s.ok) {
        throw new Error('戦略を取得できませんでした');
      }
      const sData = await s.json();
      const vData = v.ok ? await v.json() : { data: [] };
      const runnable = (vData.data as StrategyVersion[]).filter(
        x => x.strategy_type
      );
      setStrategy(sData.data);
      setVersions(runnable);
      const active = runnable.find(x => x.is_active) ?? runnable[0];
      setForm(prev => ({
        ...prev,
        name: `${sData.data.name} バックテスト`,
        strategy_version_id: active?.id ?? '',
      }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setLoading(false);
    }
  }, [strategyId]);

  useEffect(() => {
    if (strategyId) {
      load();
    }
  }, [strategyId, load]);

  const set = (field: keyof typeof form, value: string) =>
    setForm(prev => ({ ...prev, [field]: value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setRunning(true);
    setError(null);

    try {
      const response = await fetch('/api/v1/backtests', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: form.name,
          strategy_id: strategyId,
          strategy_version_id: form.strategy_version_id,
          symbols: [form.symbol.trim()],
          timeframe: form.timeframe,
          start_date: form.start_date,
          end_date: form.end_date,
          units: Number(form.units),
          initial_balance: Number(form.initial_balance),
        }),
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || 'バックテストを作成できませんでした');
      }
      router.push(`/backtests/${data.data.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setRunning(false);
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

  const selected = versions.find(v => v.id === form.strategy_version_id);

  return (
    <div className="container mx-auto p-6 max-w-2xl">
      <div className="mb-6">
        <Link
          href={`/strategies/${strategyId}`}
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          戦略へ戻る
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>バックテストを実行</CardTitle>
          <CardDescription>
            {strategy?.name} を保存済みの過去データで検証します。実行は bot
            が行い、完了すると結果ページに表示されます。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {versions.length === 0 ? (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">
                実行できるバージョンがありません。先に戦略の種類とパラメータを選んでバージョンを作成してください。
              </p>
              <Link href={`/strategies/${strategyId}/versions/new`}>
                <Button>バージョンを作成</Button>
              </Link>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-5">
              <div className="space-y-2">
                <Label htmlFor="name">名前</Label>
                <Input
                  id="name"
                  value={form.name}
                  onChange={e => set('name', e.target.value)}
                  required
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="version">バージョン</Label>
                <select
                  id="version"
                  className={selectClass}
                  value={form.strategy_version_id}
                  onChange={e => set('strategy_version_id', e.target.value)}
                >
                  {versions.map(v => (
                    <option key={v.id} value={v.id}>
                      {v.version}
                      {v.is_active ? '（有効）' : ''} — {v.strategy_type}
                    </option>
                  ))}
                </select>
                {selected && (
                  <p className="text-xs text-muted-foreground font-mono">
                    {formatParams(selected.params)}
                  </p>
                )}
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="symbol">銘柄</Label>
                  <Input
                    id="symbol"
                    value={form.symbol}
                    onChange={e => set('symbol', e.target.value.toUpperCase())}
                    placeholder="USD_JPY"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="timeframe">時間足</Label>
                  <select
                    id="timeframe"
                    className={selectClass}
                    value={form.timeframe}
                    onChange={e => set('timeframe', e.target.value)}
                  >
                    {TIMEFRAMES.map(t => (
                      <option key={t.value} value={t.value}>
                        {t.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="start_date">開始日</Label>
                  <Input
                    id="start_date"
                    type="date"
                    value={form.start_date}
                    onChange={e => set('start_date', e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="end_date">終了日（この日を含む）</Label>
                  <Input
                    id="end_date"
                    type="date"
                    value={form.end_date}
                    onChange={e => set('end_date', e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="units">数量（通貨）</Label>
                  <Input
                    id="units"
                    type="number"
                    min={1}
                    value={form.units}
                    onChange={e => set('units', e.target.value)}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="initial_balance">初期資金（円）</Label>
                  <Input
                    id="initial_balance"
                    type="number"
                    min={1}
                    value={form.initial_balance}
                    onChange={e => set('initial_balance', e.target.value)}
                    required
                  />
                </div>
              </div>

              <p className="text-xs text-muted-foreground">
                過去データは `make backfill`
                で取得したものを使います（銘柄・時間足ごとに必要）。
              </p>

              {error && (
                <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                  <p className="text-red-600 text-sm">{error}</p>
                </div>
              )}

              <Button type="submit" disabled={running}>
                <Play className="w-4 h-4 mr-2" />
                {running ? '送信中...' : '実行'}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
