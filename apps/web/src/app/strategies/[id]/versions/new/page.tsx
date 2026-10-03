'use client';

import { useState, useEffect } from 'react';
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
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { ArrowLeft, Save } from 'lucide-react';
import Link from 'next/link';
import type { StrategyType } from '@/lib/backtest';

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm';

export default function NewVersionPage() {
  const params = useParams();
  const router = useRouter();
  const strategyId = params.id as string;

  const [types, setTypes] = useState<StrategyType[]>([]);
  const [typeName, setTypeName] = useState('');
  const [values, setValues] = useState<Record<string, string>>({});
  const [version, setVersion] = useState('1.0.0');
  const [description, setDescription] = useState('');
  const [isActive, setIsActive] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch('/api/v1/strategy-types')
      .then(r => (r.ok ? r.json() : Promise.reject(new Error())))
      .then(d => {
        const list = d.data as StrategyType[];
        setTypes(list);
        if (list[0]) {
          selectType(list[0]);
        }
      })
      .catch(() => setError('戦略の種類を取得できませんでした'));
  }, []);

  const selectType = (t: StrategyType) => {
    setTypeName(t.type);
    setValues(
      Object.fromEntries(t.params.map(p => [p.name, String(p.default)]))
    );
  };

  const current = types.find(t => t.type === typeName);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);

    try {
      const numeric = Object.fromEntries(
        Object.entries(values).map(([k, v]) => [k, Number(v)])
      );
      const response = await fetch(
        `/api/v1/strategies/${strategyId}/versions`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            version,
            strategy_type: typeName,
            params: numeric,
            description: description || null,
            is_active: isActive,
          }),
        }
      );
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || 'バージョンを作成できませんでした');
      }
      router.push(`/strategies/${strategyId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setSaving(false);
    }
  };

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
          <CardTitle>新しいバージョン</CardTitle>
          <CardDescription>
            戦略の種類とパラメータを選びます。作成後は変更できません（変えるときは新しいバージョンを作ります）。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="type">戦略の種類</Label>
              <select
                id="type"
                className={selectClass}
                value={typeName}
                onChange={e => {
                  const t = types.find(x => x.type === e.target.value);
                  if (t) {
                    selectType(t);
                  }
                }}
              >
                {types.map(t => (
                  <option key={t.type} value={t.type}>
                    {t.name}（{t.type}）
                  </option>
                ))}
              </select>
              {current && (
                <p className="text-sm text-muted-foreground">
                  {current.description}
                </p>
              )}
            </div>

            {current && (
              <div className="grid gap-4 md:grid-cols-2">
                {current.params.map(p => (
                  <div key={p.name} className="space-y-2">
                    <Label htmlFor={p.name}>{p.label}</Label>
                    <Input
                      id={p.name}
                      type="number"
                      min={p.min}
                      max={p.max}
                      step={p.integer ? 1 : 'any'}
                      value={values[p.name] ?? ''}
                      onChange={e =>
                        setValues(prev => ({
                          ...prev,
                          [p.name]: e.target.value,
                        }))
                      }
                      required
                    />
                    <p className="text-xs text-muted-foreground">
                      {p.name}（{p.min}〜{p.max}、既定 {p.default}）
                    </p>
                  </div>
                ))}
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="version">バージョン名</Label>
              <Input
                id="version"
                value={version}
                onChange={e => setVersion(e.target.value)}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">メモ</Label>
              <Textarea
                id="description"
                value={description}
                onChange={e => setDescription(e.target.value)}
                placeholder="変更の理由など"
                rows={2}
              />
            </div>

            <div className="flex items-center space-x-2">
              <Switch
                id="is_active"
                checked={isActive}
                onCheckedChange={(checked: boolean) => setIsActive(checked)}
              />
              <Label htmlFor="is_active">
                このバージョンを有効にする（他のバージョンは無効になります）
              </Label>
            </div>

            {error && (
              <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                <p className="text-red-600 text-sm">{error}</p>
              </div>
            )}

            <Button type="submit" disabled={saving || !current}>
              <Save className="w-4 h-4 mr-2" />
              {saving ? '保存中...' : '作成'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
