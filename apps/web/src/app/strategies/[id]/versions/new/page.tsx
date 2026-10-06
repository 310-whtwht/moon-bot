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
import { ArrowLeft, CheckCircle, Save } from 'lucide-react';
import Link from 'next/link';
import type { ParamSpec, StrategyType } from '@/lib/backtest';
import { SCRIPT_TEMPLATE } from '@/lib/scriptTemplate';

const selectClass =
  'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm';

export default function NewVersionPage() {
  const params = useParams();
  const router = useRouter();
  const strategyId = params.id as string;

  const [types, setTypes] = useState<StrategyType[]>([]);
  const [typeName, setTypeName] = useState('');
  const [values, setValues] = useState<Record<string, string>>({});
  const [script, setScript] = useState(SCRIPT_TEMPLATE);
  // Parameters declared by the script, known once it has been checked.
  const [scriptParams, setScriptParams] = useState<ParamSpec[] | null>(null);
  const [checking, setChecking] = useState(false);
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
  const scripted = current?.scripted ?? false;
  const paramSpecs = scripted ? (scriptParams ?? []) : (current?.params ?? []);

  /** Compiles the script on the server and loads the parameters it declares. */
  const checkScript = async (): Promise<boolean> => {
    setChecking(true);
    setError(null);
    try {
      const response = await fetch('/api/v1/strategies/validate-script', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ script }),
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || 'スクリプトを確認できませんでした');
      }
      const specs = data.data.params as ParamSpec[];
      setScriptParams(specs);
      // Keep what was typed for parameters that still exist.
      setValues(prev =>
        Object.fromEntries(
          specs.map(p => [p.name, prev[p.name] ?? String(p.default)])
        )
      );
      return true;
    } catch (err) {
      setScriptParams(null);
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
      return false;
    } finally {
      setChecking(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (scripted && scriptParams === null && !(await checkScript())) {
      return;
    }
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
            script: scripted ? script : undefined,
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

            {scripted && (
              <div className="space-y-2">
                <Label htmlFor="script">スクリプト</Label>
                <Textarea
                  id="script"
                  value={script}
                  onChange={e => {
                    setScript(e.target.value);
                    setScriptParams(null);
                  }}
                  rows={22}
                  spellCheck={false}
                  className="font-mono text-xs leading-relaxed"
                />
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={checkScript}
                    disabled={checking}
                  >
                    <CheckCircle className="w-4 h-4 mr-2" />
                    {checking ? '確認中...' : 'スクリプトを確認'}
                  </Button>
                  <span className="text-sm text-muted-foreground">
                    {scriptParams === null
                      ? '確認すると、文法エラーの有無とパラメータが分かります'
                      : `問題ありません（パラメータ ${scriptParams.length} 個）`}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground">
                  使える関数: ema / sma / rsi / atr / highest / lowest（期間,
                  ago=何本前）、close / open / high / low（ago=）、buy / sell
                  （stop=損切り価格）、exit、hold、explain、num、state。bar には
                  time / open / high / low / close / hour / minute /
                  weekday（日本時間、月曜=0）があります。作成したら、有効にする前にバックテストで確かめてください。
                </p>
              </div>
            )}

            {current && paramSpecs.length > 0 && (
              <div className="grid gap-4 md:grid-cols-2">
                {paramSpecs.map(p => (
                  <div key={p.name} className="space-y-2">
                    <Label htmlFor={p.name}>{p.label}</Label>
                    <Input
                      id={p.name}
                      type="number"
                      min={scripted ? undefined : p.min}
                      max={scripted ? undefined : p.max}
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
                      {scripted
                        ? `既定 ${p.default}`
                        : `${p.name}（${p.min}〜${p.max}、既定 ${p.default}）`}
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
