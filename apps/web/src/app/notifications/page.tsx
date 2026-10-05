'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  AlertTriangle,
  Bell,
  CheckCircle,
  Info,
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
import { Spinner } from '@/components/ui/spinner';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  BOT_STATUS_CHANGED,
  type BotStatus,
  type Position,
  SCOPE_LABELS,
  fetchBotStatus,
  formatYenSigned,
} from '@/lib/bot';

const REFRESH_MS = 15000;

type Tone = 'success' | 'info' | 'warning' | 'error';

interface Notice {
  id: string;
  tone: Tone;
  category: '取引' | '安全装置' | 'システム';
  title: string;
  message: string;
  /** ISO time, or null for a state that has no moment (shown first). */
  time: string | null;
}

const dateTime = (iso: string) =>
  new Date(iso).toLocaleString('ja-JP', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
const price = (v: number) => v.toFixed(3);
const pair = (symbol: string) => symbol.replace('_', '/');
const sideLabel = (side: Position['side']) =>
  side === 'buy' ? '買い' : '売り';

/** Builds the list from what the bot has recorded: nothing here is sample data. */
function buildNotices(status: BotStatus): Notice[] {
  const notices: Notice[] = [];
  const deployment = (id: string | null) =>
    status.deployments.find(d => d.id === id)?.name;

  if (!status.bot_alive) {
    const lastSeen = status.heartbeats[0]?.last_seen_at ?? null;
    notices.push({
      id: 'bot-down',
      tone: 'error',
      category: 'システム',
      title: 'Bot が停止しています',
      message: lastSeen
        ? `最後に動作を確認したのは ${dateTime(lastSeen)} です。売買と損切りの監視が止まっています。`
        : 'まだ一度も起動していません。',
      time: null,
    });
  }

  status.kill_switches
    .filter(k => k.active)
    .forEach(k =>
      notices.push({
        id: `kill-${k.scope}`,
        tone: 'warning',
        category: '安全装置',
        title: `Kill Switch が発動中です（${SCOPE_LABELS[k.scope] ?? k.scope}）`,
        message: [
          '新規の発注を停止しています。',
          k.close_positions ? '保有中の建玉も成行で決済します。' : '',
          k.reason ? `理由: ${k.reason}` : '',
        ]
          .filter(Boolean)
          .join(' '),
        time: k.updated_at,
      })
    );

  [...status.open_positions, ...status.closed_positions].forEach(p => {
    const name = deployment(p.deployment_id);
    notices.push({
      id: `open-${p.id}`,
      tone: 'info',
      category: '取引',
      title: `新規約定: ${pair(p.symbol)} ${sideLabel(p.side)} ${p.quantity.toLocaleString()} 通貨`,
      message: [
        `建値 ${price(p.open_price)}`,
        p.stop_price != null ? `損切り ${price(p.stop_price)}` : '',
        name ? `（${name}）` : '',
      ]
        .filter(Boolean)
        .join('、'),
      time: p.opened_at,
    });
    if (p.status === 'closed' && p.closed_at) {
      const pnl = p.realized_pnl ?? 0;
      notices.push({
        id: `close-${p.id}`,
        tone: pnl >= 0 ? 'success' : 'warning',
        category: '取引',
        title: `決済: ${pair(p.symbol)} ${sideLabel(p.side)}の建玉、損益 ${formatYenSigned(pnl)}`,
        message: `建値 ${price(p.open_price)} → 決済値 ${p.close_price != null ? price(p.close_price) : '—'}`,
        time: p.closed_at,
      });
    }
  });

  return notices.sort((a, b) => {
    if (a.time === null || b.time === null) {
      return a.time === b.time ? 0 : a.time === null ? -1 : 1;
    }
    return Date.parse(b.time) - Date.parse(a.time);
  });
}

const TONE_STYLES: Record<Tone, string> = {
  success: 'border-green-200 bg-green-50',
  info: '',
  warning: 'border-yellow-200 bg-yellow-50',
  error: 'border-red-200 bg-red-50',
};

function ToneIcon({ tone }: { tone: Tone }) {
  switch (tone) {
    case 'success':
      return <CheckCircle className="w-4 h-4 mt-1 text-green-600" />;
    case 'warning':
      return <AlertTriangle className="w-4 h-4 mt-1 text-yellow-600" />;
    case 'error':
      return <XCircle className="w-4 h-4 mt-1 text-red-600" />;
    default:
      return <Info className="w-4 h-4 mt-1 text-blue-600" />;
  }
}

export default function NotificationsPage() {
  const [status, setStatus] = useState<BotStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setStatus(await fetchBotStatus());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取得できませんでした');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, REFRESH_MS);
    window.addEventListener(BOT_STATUS_CHANGED, refresh);
    return () => {
      clearInterval(timer);
      window.removeEventListener(BOT_STATUS_CHANGED, refresh);
    };
  }, [refresh]);

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <Spinner className="h-64" />
      </div>
    );
  }

  const notices = status ? buildNotices(status) : [];
  const alerts = notices.filter(
    n => n.tone === 'error' || n.category === '安全装置'
  ).length;

  return (
    <div className="container mx-auto p-6 space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold">通知</h1>
          <p className="text-muted-foreground mt-1">
            bot が記録した約定・決済と、注意が必要な状態（15秒ごとに更新）
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={refresh}>
          <RefreshCw className="w-4 h-4 mr-2" />
          更新
        </Button>
      </div>

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-md text-red-700 text-sm">
          通知を取得できませんでした: {error}
        </div>
      )}

      <Tabs defaultValue="notifications" className="w-full">
        <TabsList>
          <TabsTrigger value="notifications">
            通知
            {alerts > 0 && (
              <Badge variant="destructive" className="ml-2">
                {alerts}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="delivery">受け取り方</TabsTrigger>
        </TabsList>

        <TabsContent value="notifications" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>最近の通知</CardTitle>
              <CardDescription>
                保有中の建玉と直近10件の決済、現在の Bot・Kill Switch
                の状態から作っています
              </CardDescription>
            </CardHeader>
            <CardContent>
              {notices.length === 0 ? (
                <div className="text-center py-8">
                  <Bell className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">
                    通知はありません
                  </h3>
                  <p className="text-muted-foreground">
                    まだ約定はなく、Bot は正常に動いています
                  </p>
                </div>
              ) : (
                <div className="space-y-3">
                  {notices.map(notice => (
                    <div
                      key={notice.id}
                      className={`flex items-start gap-3 p-4 border rounded-lg ${TONE_STYLES[notice.tone]}`}
                    >
                      <ToneIcon tone={notice.tone} />
                      <div className="flex-1">
                        <div className="flex flex-wrap items-center gap-2 mb-1">
                          <h4 className="font-medium">{notice.title}</h4>
                          <Badge variant="outline">{notice.category}</Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">
                          {notice.message}
                        </p>
                        <p className="text-xs text-muted-foreground mt-1">
                          {notice.time ? dateTime(notice.time) : '現在'}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="delivery" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Slack で受け取る</CardTitle>
              <CardDescription>
                画面を開いていなくても、その場で気づけるようにします
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <p>
                Slack で Incoming Webhook を発行し、bot を動かしている環境の{' '}
                <code className="px-1 rounded bg-muted">.env</code> に{' '}
                <code className="px-1 rounded bg-muted">SLACK_WEBHOOK_URL</code>{' '}
                を設定して bot を再起動すると、次の内容が届きます。
              </p>
              <ul className="list-disc pl-5 space-y-1 text-muted-foreground">
                <li>新規約定と決済（価格・損切り・損益）</li>
                <li>発注の失敗や拒否、照合で見つかった食い違い</li>
                <li>Kill Switch の発動と解除</li>
                <li>日次サマリ（毎朝6時）</li>
              </ul>
              <p className="text-muted-foreground">
                設定されているかどうかは、この画面からは確認できません。メールと
                Webhook での通知には対応していません。
              </p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
