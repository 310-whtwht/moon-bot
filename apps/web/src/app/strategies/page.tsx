'use client';

import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Plus, Code, Settings, Play, Trash2 } from 'lucide-react';
import Link from 'next/link';

interface Strategy {
  id: string;
  name: string;
  description?: string;
  author: string;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export default function StrategiesPage() {
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchStrategies();
  }, []);

  const fetchStrategies = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/v1/strategies');
      if (!response.ok) {
        throw new Error('戦略の取得に失敗しました');
      }
      const data = await response.json();
      setStrategies(data.data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteStrategy = async (id: string) => {
    if (!window.confirm('この戦略を削除してもよろしいですか？')) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/strategies/${id}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error('戦略の削除に失敗しました');
      }
      await fetchStrategies();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : '戦略の削除に失敗しました'
      );
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">戦略を読み込み中...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">エラー: {error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-3xl font-bold">戦略管理</h1>
          <p className="text-muted-foreground">
            トレーディング戦略の管理
          </p>
        </div>
        <Link href="/strategies/new">
          <Button>
            <Plus className="w-4 h-4 mr-2" />
            新規戦略
          </Button>
        </Link>
      </div>

      {strategies.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center h-64">
            <Code className="w-12 h-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">戦略がまだありません</h3>
            <p className="text-muted-foreground mb-4">
              最初のトレーディング戦略を作成して開始してください
            </p>
            <Link href="/strategies/new">
              <Button>
                <Plus className="w-4 h-4 mr-2" />
                戦略を作成
              </Button>
            </Link>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {strategies.map(strategy => (
            <Card
              key={strategy.id}
              className="hover:shadow-lg transition-shadow"
            >
              <CardHeader>
                <div className="flex justify-between items-start">
                  <div>
                    <CardTitle className="text-lg">{strategy.name}</CardTitle>
                    <CardDescription className="mt-1">
                      {strategy.description || '説明なし'}
                    </CardDescription>
                  </div>
                  <Badge variant={strategy.is_public ? 'default' : 'secondary'}>
                    {strategy.is_public ? '公開' : '非公開'}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  <div className="text-sm text-muted-foreground">
                    <span className="font-medium">作成者:</span>{' '}
                    {strategy.author}
                  </div>
                  <div className="text-sm text-muted-foreground">
                    <span className="font-medium">作成日:</span>{' '}
                    {new Date(strategy.created_at).toLocaleDateString('ja-JP')}
                  </div>
                  <div className="flex gap-2 pt-2">
                    <Link href={`/strategies/${strategy.id}`}>
                      <Button variant="outline" size="sm">
                        <Settings className="w-4 h-4 mr-1" />
                        編集
                      </Button>
                    </Link>
                    <Link href={`/strategies/${strategy.id}/backtest`}>
                      <Button variant="outline" size="sm">
                        <Play className="w-4 h-4 mr-1" />
                        バックテスト
                      </Button>
                    </Link>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleDeleteStrategy(strategy.id)}
                      className="text-red-600 hover:text-red-700"
                    >
                      <Trash2 className="w-4 h-4 mr-1" />
                      削除
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
