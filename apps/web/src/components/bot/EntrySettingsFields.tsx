'use client';

import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { EntrySettings } from '@/lib/bot';

const selectClass =
  'h-10 w-full rounded-md border border-input bg-background px-3 text-sm';

/** What the form holds: numbers stay as typed text until they are submitted. */
export interface EntryForm {
  entry_order: EntrySettings['entry_order'];
  limit_wait_seconds: string;
  limit_fallback: EntrySettings['limit_fallback'];
  max_spread: string;
}

export function toEntryForm(e: EntrySettings): EntryForm {
  return {
    entry_order: e.entry_order,
    limit_wait_seconds: String(e.limit_wait_seconds),
    limit_fallback: e.limit_fallback,
    max_spread: e.max_spread != null ? String(e.max_spread) : '',
  };
}

export function fromEntryForm(f: EntryForm): EntrySettings {
  const spread = Number(f.max_spread);
  return {
    entry_order: f.entry_order,
    limit_wait_seconds: Number(f.limit_wait_seconds) || 30,
    limit_fallback: f.limit_fallback,
    max_spread: f.max_spread.trim() !== '' && spread > 0 ? spread : null,
  };
}

/** One line describing the settings, for lists. */
export function describeEntry(e: EntrySettings): string {
  const order =
    e.entry_order === 'limit'
      ? `指値（最大 ${e.limit_wait_seconds} 秒待ち、約定しなければ${e.limit_fallback === 'market' ? '成行で入る' : '見送る'}）`
      : '成行';
  const spread =
    e.max_spread != null ? `、スプレッド ${e.max_spread} 超は見送り` : '';
  return `新規は${order}${spread}`;
}

/** The fields for how a deployment sends its entries. */
export function EntrySettingsFields({
  idPrefix,
  value,
  onChange,
}: {
  idPrefix: string;
  value: EntryForm;
  onChange: (next: EntryForm) => void;
}) {
  const limit = value.entry_order === 'limit';
  return (
    <div className="space-y-2">
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-1">
          <Label htmlFor={`${idPrefix}-entry-order`}>新規の発注方法</Label>
          <select
            id={`${idPrefix}-entry-order`}
            className={selectClass}
            value={value.entry_order}
            onChange={e =>
              onChange({
                ...value,
                entry_order: e.target.value as EntryForm['entry_order'],
              })
            }
          >
            <option value="market">成行（すぐ約定）</option>
            <option value="limit">指値（有利な側で待つ）</option>
          </select>
        </div>
        <div className="space-y-1">
          <Label htmlFor={`${idPrefix}-limit-wait`}>指値の待ち時間（秒）</Label>
          <Input
            id={`${idPrefix}-limit-wait`}
            type="number"
            min={5}
            max={300}
            step={1}
            disabled={!limit}
            value={value.limit_wait_seconds}
            onChange={e =>
              onChange({ ...value, limit_wait_seconds: e.target.value })
            }
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor={`${idPrefix}-limit-fallback`}>約定しなかったら</Label>
          <select
            id={`${idPrefix}-limit-fallback`}
            className={selectClass}
            disabled={!limit}
            value={value.limit_fallback}
            onChange={e =>
              onChange({
                ...value,
                limit_fallback: e.target.value as EntryForm['limit_fallback'],
              })
            }
          >
            <option value="skip">見送る</option>
            <option value="market">成行で入る</option>
          </select>
        </div>
        <div className="space-y-1">
          <Label htmlFor={`${idPrefix}-max-spread`}>スプレッドの上限</Label>
          <Input
            id={`${idPrefix}-max-spread`}
            type="number"
            min={0}
            step="any"
            placeholder="空欄 = 制限なし"
            value={value.max_spread}
            onChange={e => onChange({ ...value, max_spread: e.target.value })}
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">
        指値は、買いなら BID・売りなら ASK
        の価格に注文を置き、待ち時間のあいだ約定を待ちます（5〜300秒）。約定しなければ取り消します。スプレッドの上限は価格の幅で指定します（USD/JPY
        で 0.02 なら
        2銭）。上限より広いときは、その新規を見送ります。決済と損切りは常に成行です。
      </p>
    </div>
  );
}
