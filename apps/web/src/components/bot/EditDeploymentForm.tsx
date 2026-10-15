'use client';

import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { type Deployment, updateDeployment } from '@/lib/bot';
import {
  EntrySettingsFields,
  fromEntryForm,
  toEntryForm,
} from './EntrySettingsFields';

/** Changes a deployment's strategy, size and how it sends entries. */
export function EditDeploymentForm({
  deployment,
  onSaved,
  onCancel,
}: {
  deployment: Deployment;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const [strategies, setStrategies] = useState<{ id: string; name: string }[]>(
    []
  );
  const [strategyId, setStrategyId] = useState(deployment.strategy_id);
  const [name, setName] = useState(deployment.name);
  const [units, setUnits] = useState(String(deployment.units));
  const [entry, setEntry] = useState(toEntryForm(deployment));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch('/api/v1/strategies?limit=100', { cache: 'no-store' })
      .then(r => r.json())
      .then(body => setStrategies(body.data ?? []))
      .catch(() => undefined);
  }, []);

  const changingStrategy = strategyId !== deployment.strategy_id;

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await updateDeployment(deployment.id, {
        name: name.trim() || deployment.name,
        units: Number(units),
        strategy_id: strategyId,
        ...fromEntryForm(entry),
      });
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存できませんでした');
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="mt-4 pt-4 border-t space-y-4">
      <div className="space-y-1 max-w-md">
        <Label htmlFor={`edit-${deployment.id}-name`}>名前</Label>
        <Input
          id={`edit-${deployment.id}-name`}
          value={name}
          onChange={e => setName(e.target.value)}
        />
      </div>
      <div className="space-y-1 max-w-md">
        <Label htmlFor={`edit-${deployment.id}-strategy`}>戦略</Label>
        <select
          id={`edit-${deployment.id}-strategy`}
          className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
          value={strategyId}
          onChange={e => setStrategyId(e.target.value)}
        >
          {strategies.length === 0 && (
            <option value={deployment.strategy_id}>
              {deployment.strategy_name}
            </option>
          )}
          {strategies.map(s => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
        {changingStrategy && (
          <p className="text-xs text-yellow-700">
            戦略の有効なバージョンで売買します。建玉を保有中なら、その建玉は今の戦略が決済まで判断し、決済後に新しい戦略に切り替わります。名前は自動では変わらないので、必要なら上の欄で変えてください。
          </p>
        )}
      </div>
      <div className="space-y-1 max-w-xs">
        <Label htmlFor={`edit-${deployment.id}-units`}>数量（通貨）</Label>
        <Input
          id={`edit-${deployment.id}-units`}
          type="number"
          min={1}
          step="any"
          value={units}
          onChange={e => setUnits(e.target.value)}
          required
        />
      </div>
      <EntrySettingsFields
        idPrefix={`edit-${deployment.id}`}
        value={entry}
        onChange={setEntry}
      />
      <p className="text-xs text-muted-foreground">
        変更は次の新規から使われます。保有中の建玉と、待機中の指値には影響しません。銘柄・足を変えるときは、削除して作り直します。
      </p>
      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          {error}
        </div>
      )}
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={saving}>
          {saving ? '保存中...' : '保存'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onCancel}>
          キャンセル
        </Button>
      </div>
    </form>
  );
}
