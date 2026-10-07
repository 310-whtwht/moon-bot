'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { type Deployment, updateDeployment } from '@/lib/bot';
import {
  EntrySettingsFields,
  fromEntryForm,
  toEntryForm,
} from './EntrySettingsFields';

/** Changes a deployment's size and how it sends entries. */
export function EditDeploymentForm({
  deployment,
  onSaved,
  onCancel,
}: {
  deployment: Deployment;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const [units, setUnits] = useState(String(deployment.units));
  const [entry, setEntry] = useState(toEntryForm(deployment));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await updateDeployment(deployment.id, {
        units: Number(units),
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
        変更は次の新規から使われます。保有中の建玉と、待機中の指値には影響しません。銘柄・足・戦略を変えるときは、削除して作り直します。
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
