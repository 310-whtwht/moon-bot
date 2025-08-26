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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  DollarSign,
  Activity,
  AlertTriangle,
  CheckCircle,
  Clock,
  BarChart3,
  Target,
  Zap,
  Shield,
} from 'lucide-react';

interface DashboardStats {
  totalPnL: number;
  dailyPnL: number;
  totalTrades: number;
  winRate: number;
  maxDrawdown: number;
  sharpeRatio: number;
  activeStrategies: number;
  totalStrategies: number;
  activeOrders: number;
  pendingOrders: number;
  systemHealth: 'healthy' | 'warning' | 'error';
  lastUpdate: string;
}

interface Strategy {
  id: string;
  name: string;
  status: 'active' | 'paused' | 'stopped';
  pnl: number;
  trades: number;
  winRate: number;
  lastTrade: string;
}

interface Order {
  id: string;
  symbol: string;
  side: string;
  status: string;
  quantity: number;
  price?: number;
  created_at: string;
}

interface Alert {
  id: string;
  level: 'info' | 'warning' | 'error';
  message: string;
  timestamp: string;
  category: string;
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [recentOrders, setRecentOrders] = useState<Order[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchDashboardData();
    // Set up real-time updates every 30 seconds
    const interval = setInterval(fetchDashboardData, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchDashboardData = async () => {
    try {
      setLoading(true);

      // Fetch strategies
      const strategiesResponse = await fetch('/api/v1/strategies');
      if (strategiesResponse.ok) {
        const strategiesData = await strategiesResponse.json();
        setStrategies(strategiesData.data || []);
      }

      // Fetch recent orders
      const ordersResponse = await fetch('/api/v1/orders?limit=10');
      if (ordersResponse.ok) {
        const ordersData = await ordersResponse.json();
        setRecentOrders(ordersData.data || []);
      }

      // Mock dashboard stats for now
      setStats({
        totalPnL: 15420.5,
        dailyPnL: 1250.75,
        totalTrades: 342,
        winRate: 68.5,
        maxDrawdown: -8.2,
        sharpeRatio: 1.85,
        activeStrategies: 3,
        totalStrategies: 5,
        activeOrders: 2,
        pendingOrders: 1,
        systemHealth: 'healthy',
        lastUpdate: new Date().toISOString(),
      });

      // Mock alerts
      setAlerts([
        {
          id: '1',
          level: 'info',
          message: '戦略「EMA Cross」が本日5回の取引を実行しました',
          timestamp: new Date(Date.now() - 300000).toISOString(),
          category: 'strategy',
        },
        {
          id: '2',
          level: 'warning',
          message: 'AAPLで高ボラティリティが検出されました',
          timestamp: new Date(Date.now() - 600000).toISOString(),
          category: 'market',
        },
        {
          id: '3',
          level: 'error',
          message: 'moomoo APIへの接続が失われました',
          timestamp: new Date(Date.now() - 900000).toISOString(),
          category: 'system',
        },
      ]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'エラーが発生しました');
    } finally {
      setLoading(false);
    }
  };

  const getHealthColor = (health: string) => {
    switch (health) {
      case 'healthy':
        return 'bg-green-100 text-green-800';
      case 'warning':
        return 'bg-yellow-100 text-yellow-800';
      case 'error':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getHealthIcon = (health: string) => {
    switch (health) {
      case 'healthy':
        return <CheckCircle className="w-4 h-4" />;
      case 'warning':
        return <AlertTriangle className="w-4 h-4" />;
      case 'error':
        return <AlertTriangle className="w-4 h-4" />;
      default:
        return <Clock className="w-4 h-4" />;
    }
  };

  const getAlertIcon = (level: string) => {
    switch (level) {
      case 'info':
        return <CheckCircle className="w-4 h-4 text-blue-600" />;
      case 'warning':
        return <AlertTriangle className="w-4 h-4 text-yellow-600" />;
      case 'error':
        return <AlertTriangle className="w-4 h-4 text-red-600" />;
      default:
        return <Activity className="w-4 h-4" />;
    }
  };

  const formatCurrency = (num: number) => {
    return new Intl.NumberFormat('ja-JP', {
      style: 'currency',
      currency: 'USD',
    }).format(num);
  };

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">ダッシュボードを読み込み中...</div>
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
          <h1 className="text-3xl font-bold">トレーディングダッシュボード</h1>
          <p className="text-muted-foreground">
            リアルタイム監視とパフォーマンス概要
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={fetchDashboardData}>
            <Activity className="w-4 h-4 mr-2" />
            更新
          </Button>
        </div>
      </div>

      {/* Key Metrics */}
      <div className="grid gap-4 mb-6 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  総損益
                </p>
                <p
                  className={`text-2xl font-bold ${stats?.totalPnL && stats.totalPnL >= 0 ? 'text-green-600' : 'text-red-600'}`}
                >
                  {stats?.totalPnL ? formatCurrency(stats.totalPnL) : '$0.00'}
                </p>
                <p
                  className={`text-sm ${stats?.dailyPnL && stats.dailyPnL >= 0 ? 'text-green-600' : 'text-red-600'}`}
                >
                  {stats?.dailyPnL
                    ? `本日 ${formatCurrency(stats.dailyPnL)}`
                    : '本日 $0.00'}
                </p>
              </div>
              <DollarSign className="w-8 h-8 text-muted-foreground" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  勝率
                </p>
                <p className="text-2xl font-bold">
                  {stats?.winRate ? `${stats.winRate}%` : '0%'}
                </p>
                <p className="text-sm text-muted-foreground">
                  {stats?.totalTrades
                    ? `${stats.totalTrades} 取引`
                    : '0 取引'}
                </p>
              </div>
              <Target className="w-8 h-8 text-blue-600" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  シャープレシオ
                </p>
                <p className="text-2xl font-bold">
                  {stats?.sharpeRatio ? stats.sharpeRatio.toFixed(2) : '0.00'}
                </p>
                <p className="text-sm text-muted-foreground">
                  最大ドローダウン: {stats?.maxDrawdown ? `${stats.maxDrawdown}%` : '0%'}
                </p>
              </div>
              <BarChart3 className="w-8 h-8 text-purple-600" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">
                  システム状態
                </p>
                <div className="flex items-center gap-2 mt-1">
                  {stats?.systemHealth && getHealthIcon(stats.systemHealth)}
                  <Badge
                    className={
                      stats?.systemHealth
                        ? getHealthColor(stats.systemHealth)
                        : 'bg-gray-100 text-gray-800'
                    }
                  >
                    {stats?.systemHealth === 'healthy' ? '正常' : 
                     stats?.systemHealth === 'warning' ? '警告' :
                     stats?.systemHealth === 'error' ? 'エラー' : '不明'}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mt-1">
                  {stats?.activeStrategies
                    ? `${stats.activeStrategies}/${stats.totalStrategies} アクティブ`
                    : '0/0 アクティブ'}
                </p>
              </div>
              <Shield className="w-8 h-8 text-green-600" />
            </div>
          </CardContent>
        </Card>
      </div>

      <Tabs defaultValue="overview" className="w-full">
        <TabsList>
          <TabsTrigger value="overview">概要</TabsTrigger>
          <TabsTrigger value="strategies">戦略</TabsTrigger>
          <TabsTrigger value="orders">最近の注文</TabsTrigger>
          <TabsTrigger value="alerts">アラート</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-6">
          <div className="grid gap-6 md:grid-cols-2">
            {/* Performance Chart Placeholder */}
            <Card>
              <CardHeader>
                <CardTitle>パフォーマンスチャート</CardTitle>
                <CardDescription>時系列での日次損益</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="h-64 flex items-center justify-center bg-muted rounded-lg">
                  <div className="text-center">
                    <BarChart3 className="w-12 h-12 text-muted-foreground mx-auto mb-2" />
                    <p className="text-muted-foreground">
                      チャートコンポーネントは実装予定です
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Active Orders Summary */}
            <Card>
              <CardHeader>
                <CardTitle>アクティブ注文</CardTitle>
                <CardDescription>現在の注文状況</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">アクティブ注文:</span>
                    <Badge variant="outline">{stats?.activeOrders || 0}</Badge>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">保留中注文:</span>
                    <Badge variant="outline">{stats?.pendingOrders || 0}</Badge>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">
                      総戦略数:
                    </span>
                    <Badge variant="outline">
                      {stats?.totalStrategies || 0}
                    </Badge>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="strategies" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>アクティブ戦略</CardTitle>
              <CardDescription>リアルタイム戦略パフォーマンス</CardDescription>
            </CardHeader>
            <CardContent>
              {strategies.length === 0 ? (
                <div className="text-center py-8">
                  <Zap className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">
                    戦略が見つかりません
                  </h3>
                  <p className="text-muted-foreground">
                    最初の戦略を作成して開始してください
                  </p>
                </div>
              ) : (
                <div className="space-y-4">
                  {strategies.map(strategy => (
                    <div
                      key={strategy.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <h4 className="font-medium">{strategy.name}</h4>
                          <Badge
                            variant={
                              strategy.status === 'active'
                                ? 'default'
                                : 'secondary'
                            }
                          >
                            {strategy.status === 'active' ? 'アクティブ' :
                             strategy.status === 'paused' ? '一時停止' :
                             strategy.status === 'stopped' ? '停止' : strategy.status}
                          </Badge>
                        </div>
                        <div className="grid grid-cols-3 gap-4 text-sm text-muted-foreground">
                          <div>
                            <span className="font-medium">損益:</span>{' '}
                            {formatCurrency(strategy.pnl)}
                          </div>
                          <div>
                            <span className="font-medium">取引数:</span>{' '}
                            {strategy.trades}
                          </div>
                          <div>
                            <span className="font-medium">勝率:</span>{' '}
                            {strategy.winRate}%
                          </div>
                        </div>
                      </div>
                      <div className="text-right text-sm text-muted-foreground">
                        最終取引: {' '}
                        {new Date(strategy.lastTrade).toLocaleDateString('ja-JP')}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="orders" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>最近の注文</CardTitle>
              <CardDescription>最新の注文アクティビティ</CardDescription>
            </CardHeader>
            <CardContent>
              {recentOrders.length === 0 ? (
                <div className="text-center py-8">
                  <Activity className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">
                    最近の注文がありません
                  </h3>
                  <p className="text-muted-foreground">
                    注文が発注されるとここに表示されます
                  </p>
                </div>
              ) : (
                <div className="space-y-4">
                  {recentOrders.map(order => (
                    <div
                      key={order.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <h4 className="font-medium">{order.symbol}</h4>
                          <Badge
                            className={
                              order.side === 'buy'
                                ? 'bg-green-100 text-green-800'
                                : 'bg-red-100 text-red-800'
                            }
                          >
                            {order.side === 'buy' ? '買い' : '売り'}
                          </Badge>
                          <Badge variant="outline">{order.status}</Badge>
                        </div>
                        <div className="text-sm text-muted-foreground">
                          {order.quantity} 株{' '}
                          {order.price && `@ ${formatCurrency(order.price)}`}
                        </div>
                      </div>
                      <div className="text-right text-sm text-muted-foreground">
                        {new Date(order.created_at).toLocaleString('ja-JP')}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="alerts" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>システムアラート</CardTitle>
              <CardDescription>
                最近のシステム通知と警告
              </CardDescription>
            </CardHeader>
            <CardContent>
              {alerts.length === 0 ? (
                <div className="text-center py-8">
                  <CheckCircle className="w-12 h-12 text-green-600 mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">
                    すべてのシステムが正常に動作中
                  </h3>
                  <p className="text-muted-foreground">
                    現在アラートはありません
                  </p>
                </div>
              ) : (
                <div className="space-y-4">
                  {alerts.map(alert => (
                    <div
                      key={alert.id}
                      className="flex items-start gap-3 p-4 border rounded-lg"
                    >
                      {getAlertIcon(alert.level)}
                      <div className="flex-1">
                        <p className="text-sm font-medium">{alert.message}</p>
                        <div className="flex items-center gap-2 mt-1">
                          <Badge variant="outline" className="text-xs">
                            {alert.category === 'strategy' ? '戦略' :
                             alert.category === 'market' ? '市場' :
                             alert.category === 'system' ? 'システム' : alert.category}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            {new Date(alert.timestamp).toLocaleString('ja-JP')}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
