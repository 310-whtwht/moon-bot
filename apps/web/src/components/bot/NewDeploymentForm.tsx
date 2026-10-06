'use client';

import { useEffect, useState } from 'react';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { type Instrument, createDeployment, fetchInstruments } from '@/lib/bot';

const TIMEFRAMES = [
  { value: '1m', label: '1分足' },
  { value: '5m', label: '5分足' },
  { value: '15m', label: '15分足' },
  { value: '30m', label: '30分足' },
  { value: '1h', label: '1時間足' },
  { value: '4h', label: '4時間足' },
  { value: '1d', label: '日足' },
];

interface StrategyOption {
  id: string;
  name: string;
}

const selectClass =
  'h-10 w-full rounded-md border border-input bg-background px-3 text-sm';

/** Adds a deployment on the paper account. It starts disabled. */
export function NewDeploymentForm({
  takenSymbols,
  onCreated,
}: {
  /** Symbols that already have a deployment (one per account and symbol). */
  takenSymbols: string[];
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [strategies, setStrategies] = useState<StrategyOption[]>([]);
  const [instruments, setInstruments] = useState<Instrument[]>([]);
  const [strategyId, setStrategyId] = useState('');
  const [symbol, setSymbol] = useState('');
  const [timeframe, setTimeframe] = useState('15m');
  const [units, setUnits] = useState('');
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    Promise.all([
      fetch('/api/v1/strategies?limit=100', { cache: 'no-store' }).then(r =>
        r.json()
      ),
      fetchInstruments(),
    ])
      .then(([strategyBody, instrumentList]) => {
        const list: StrategyOption[] = strategyBody.data ?? [];
        setStrategies(list);
        setInstruments(instrumentList);
        setStrategyId(prev => prev || list[0]?.id || '');
      })
      .catch(err =>
        setError(
          err instanceof Error ? err.message : '選択肢を取得できませんでした'
        )
      );
  }, [open]);

  const available = instruments.filter(i => !takenSymbols.includes(i.symbol));
  const chosen = available.find(i => i.symbol === symbol) ?? available[0];
  const strategyName = strategies.find(s => s.id === strategyId)?.name ?? '';
  const timeframeLabel =
    TIMEFRAMES.find(t => t.value === timeframe)?.label ?? timeframe;
  const defaultName = chosen
    ? `${strategyName} ${chosen.symbol.replace('_', '/')} ${timeframeLabel}（Paper）`
    : '';

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!chosen) {
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await createDeployment({
        name: name.trim() || defaultName,
        strategy_id: strategyId,
        symbol: chosen.symbol,
        timeframe,
        units: Number(units || chosen.min_units),
      });
      setOpen(false);
      setName('');
      setUnits('');
      setSymbol('');
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : '作成できませんでした');
    } finally {
      setSaving(false);
    }
  };

  if (!open) {
    return (
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        <Plus className="w-4 h-4 mr-2" />
        割り当てを追加
      </Button>
    );
  }

  return (
    <form onSubmit={submit} className="p-4 border rounded-lg space-y-4">
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-1">
          <Label htmlFor="deployment-strategy">戦略</Label>
          <select
            id="deployment-strategy"
            className={selectClass}
            value={strategyId}
            onChange={e => setStrategyId(e.target.value)}
          >
            {strategies.map(s => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-1">
          <Label htmlFor="deployment-symbol">銘柄</Label>
          <select
            id="deployment-symbol"
            className={selectClass}
            value={chosen?.symbol ?? ''}
            onChange={e => {
              setSymbol(e.target.value);
              setUnits('');
            }}
          >
            {available.map(i => (
              <option key={i.symbol} value={i.symbol}>
                {i.symbol.replace('_', '/')}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-1">
          <Label htmlFor="deployment-timeframe">足</Label>
          <select
            id="deployment-timeframe"
            className={selectClass}
            value={timeframe}
            onChange={e => setTimeframe(e.target.value)}
          >
            {TIMEFRAMES.map(t => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-1">
          <Label htmlFor="deployment-units">数量（通貨）</Label>
          <Input
            id="deployment-units"
            type="number"
            min={chosen?.min_units ?? 1}
            step={chosen?.step ?? 1}
            placeholder={chosen ? String(chosen.min_units) : ''}
            value={units}
            onChange={e => setUnits(e.target.value)}
          />
        </div>
      </div>
      <div className="space-y-1">
        <Label htmlFor="deployment-name">名前（空欄なら自動）</Label>
        <Input
          id="deployment-name"
          placeholder={defaultName}
          value={name}
          onChange={e => setName(e.target.value)}
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Paper
        口座に、無効の状態で追加します。追加後にスイッチをオンにすると売買を始めます。
        {chosen && ` 最小 ${chosen.min_units.toLocaleString('ja-JP')} 通貨。`}
        円建ての通貨ペアのみ選べます（1銘柄につき1つ）。
      </p>
      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          {error}
        </div>
      )}
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving || !chosen}>
          {saving ? '追加中...' : '追加する'}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setOpen(false)}
        >
          キャンセル
        </Button>
      </div>
    </form>
  );
}
