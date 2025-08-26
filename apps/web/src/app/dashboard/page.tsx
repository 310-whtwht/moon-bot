'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { 
  TrendingUp, 
  TrendingDown, 
  DollarSign, 
  Activity, 
  AlertTriangle, 
  CheckCircle, 
  Clock, 
  BarChart3,
  Users,
  Target,
  Zap,
  Shield
} from 'lucide-react'

interface DashboardStats {
  totalPnL: number
  dailyPnL: number
  totalTrades: number
  winRate: number
  maxDrawdown: number
  sharpeRatio: number
  activeStrategies: number
  totalStrategies: number
  activeOrders: number
  pendingOrders: number
  systemHealth: 'healthy' | 'warning' | 'error'
  lastUpdate: string
}

interface Strategy {
  id: string
  name: string
  status: 'active' | 'paused' | 'stopped'
  pnl: number
  trades: number
  winRate: number
  lastTrade: string
}

interface Order {
  id: string
  symbol: string
  side: string
  status: string
  quantity: number
  price?: number
  created_at: string
}

interface Alert {
  id: string
  level: 'info' | 'warning' | 'error'
  message: string
  timestamp: string
  category: string
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [strategies, setStrategies] = useState<Strategy[]>([])
  const [recentOrders, setRecentOrders] = useState<Order[]>([])
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetchDashboardData()
    // Set up real-time updates every 30 seconds
    const interval = setInterval(fetchDashboardData, 30000)
    return () => clearInterval(interval)
  }, [])

  const fetchDashboardData = async () => {
    try {
      setLoading(true)
      
      // Fetch strategies
      const strategiesResponse = await fetch('/api/v1/strategies')
      if (strategiesResponse.ok) {
        const strategiesData = await strategiesResponse.json()
        setStrategies(strategiesData.data || [])
      }

      // Fetch recent orders
      const ordersResponse = await fetch('/api/v1/orders?limit=10')
      if (ordersResponse.ok) {
        const ordersData = await ordersResponse.json()
        setRecentOrders(ordersData.data || [])
      }

      // Mock dashboard stats for now
      setStats({
        totalPnL: 15420.50,
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
        lastUpdate: new Date().toISOString()
      })

      // Mock alerts
      setAlerts([
        {
          id: '1',
          level: 'info',
          message: 'Strategy "EMA Cross" executed 5 trades today',
          timestamp: new Date(Date.now() - 300000).toISOString(),
          category: 'strategy'
        },
        {
          id: '2',
          level: 'warning',
          message: 'High volatility detected in AAPL',
          timestamp: new Date(Date.now() - 600000).toISOString(),
          category: 'market'
        },
        {
          id: '3',
          level: 'error',
          message: 'Connection lost to moomoo API',
          timestamp: new Date(Date.now() - 900000).toISOString(),
          category: 'system'
        }
      ])

    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setLoading(false)
    }
  }

  const getHealthColor = (health: string) => {
    switch (health) {
      case 'healthy':
        return 'bg-green-100 text-green-800'
      case 'warning':
        return 'bg-yellow-100 text-yellow-800'
      case 'error':
        return 'bg-red-100 text-red-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const getHealthIcon = (health: string) => {
    switch (health) {
      case 'healthy':
        return <CheckCircle className="w-4 h-4" />
      case 'warning':
        return <AlertTriangle className="w-4 h-4" />
      case 'error':
        return <AlertTriangle className="w-4 h-4" />
      default:
        return <Clock className="w-4 h-4" />
    }
  }

  const getAlertIcon = (level: string) => {
    switch (level) {
      case 'info':
        return <CheckCircle className="w-4 h-4 text-blue-600" />
      case 'warning':
        return <AlertTriangle className="w-4 h-4 text-yellow-600" />
      case 'error':
        return <AlertTriangle className="w-4 h-4 text-red-600" />
      default:
        return <Activity className="w-4 h-4" />
    }
  }

  const formatCurrency = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(num)
  }

  const formatPercentage = (num: number) => {
    return `${num > 0 ? '+' : ''}${num.toFixed(2)}%`
  }

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading dashboard...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">Error: {error}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-3xl font-bold">Trading Dashboard</h1>
          <p className="text-muted-foreground">Real-time monitoring and performance overview</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={fetchDashboardData}>
            <Activity className="w-4 h-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Key Metrics */}
      <div className="grid gap-4 mb-6 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Total P&L</p>
                <p className={`text-2xl font-bold ${stats?.totalPnL && stats.totalPnL >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                  {stats?.totalPnL ? formatCurrency(stats.totalPnL) : '$0.00'}
                </p>
                <p className={`text-sm ${stats?.dailyPnL && stats.dailyPnL >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                  {stats?.dailyPnL ? `${formatCurrency(stats.dailyPnL)} today` : '$0.00 today'}
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
                <p className="text-sm font-medium text-muted-foreground">Win Rate</p>
                <p className="text-2xl font-bold">{stats?.winRate ? `${stats.winRate}%` : '0%'}</p>
                <p className="text-sm text-muted-foreground">
                  {stats?.totalTrades ? `${stats.totalTrades} trades` : '0 trades'}
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
                <p className="text-sm font-medium text-muted-foreground">Sharpe Ratio</p>
                <p className="text-2xl font-bold">{stats?.sharpeRatio ? stats.sharpeRatio.toFixed(2) : '0.00'}</p>
                <p className="text-sm text-muted-foreground">
                  Max DD: {stats?.maxDrawdown ? `${stats.maxDrawdown}%` : '0%'}
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
                <p className="text-sm font-medium text-muted-foreground">System Health</p>
                <div className="flex items-center gap-2 mt-1">
                  {stats?.systemHealth && getHealthIcon(stats.systemHealth)}
                  <Badge className={stats?.systemHealth ? getHealthColor(stats.systemHealth) : 'bg-gray-100 text-gray-800'}>
                    {stats?.systemHealth || 'Unknown'}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mt-1">
                  {stats?.activeStrategies ? `${stats.activeStrategies}/${stats.totalStrategies} active` : '0/0 active'}
                </p>
              </div>
              <Shield className="w-8 h-8 text-green-600" />
            </div>
          </CardContent>
        </Card>
      </div>

      <Tabs defaultValue="overview" className="w-full">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="strategies">Strategies</TabsTrigger>
          <TabsTrigger value="orders">Recent Orders</TabsTrigger>
          <TabsTrigger value="alerts">Alerts</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-6">
          <div className="grid gap-6 md:grid-cols-2">
            {/* Performance Chart Placeholder */}
            <Card>
              <CardHeader>
                <CardTitle>Performance Chart</CardTitle>
                <CardDescription>Daily P&L over time</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="h-64 flex items-center justify-center bg-muted rounded-lg">
                  <div className="text-center">
                    <BarChart3 className="w-12 h-12 text-muted-foreground mx-auto mb-2" />
                    <p className="text-muted-foreground">Chart component will be implemented</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Active Orders Summary */}
            <Card>
              <CardHeader>
                <CardTitle>Active Orders</CardTitle>
                <CardDescription>Current order status</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">Active Orders:</span>
                    <Badge variant="outline">{stats?.activeOrders || 0}</Badge>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">Pending Orders:</span>
                    <Badge variant="outline">{stats?.pendingOrders || 0}</Badge>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium">Total Strategies:</span>
                    <Badge variant="outline">{stats?.totalStrategies || 0}</Badge>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="strategies" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Active Strategies</CardTitle>
              <CardDescription>Real-time strategy performance</CardDescription>
            </CardHeader>
            <CardContent>
              {strategies.length === 0 ? (
                <div className="text-center py-8">
                  <Zap className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">No strategies found</h3>
                  <p className="text-muted-foreground">Create your first strategy to get started</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {strategies.map((strategy) => (
                    <div key={strategy.id} className="flex items-center justify-between p-4 border rounded-lg">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <h4 className="font-medium">{strategy.name}</h4>
                          <Badge variant={strategy.status === 'active' ? 'default' : 'secondary'}>
                            {strategy.status}
                          </Badge>
                        </div>
                        <div className="grid grid-cols-3 gap-4 text-sm text-muted-foreground">
                          <div>
                            <span className="font-medium">P&L:</span> {formatCurrency(strategy.pnl)}
                          </div>
                          <div>
                            <span className="font-medium">Trades:</span> {strategy.trades}
                          </div>
                          <div>
                            <span className="font-medium">Win Rate:</span> {strategy.winRate}%
                          </div>
                        </div>
                      </div>
                      <div className="text-right text-sm text-muted-foreground">
                        Last trade: {new Date(strategy.lastTrade).toLocaleDateString()}
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
              <CardTitle>Recent Orders</CardTitle>
              <CardDescription>Latest order activity</CardDescription>
            </CardHeader>
            <CardContent>
              {recentOrders.length === 0 ? (
                <div className="text-center py-8">
                  <Activity className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">No recent orders</h3>
                  <p className="text-muted-foreground">Orders will appear here when placed</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {recentOrders.map((order) => (
                    <div key={order.id} className="flex items-center justify-between p-4 border rounded-lg">
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <h4 className="font-medium">{order.symbol}</h4>
                          <Badge className={order.side === 'buy' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'}>
                            {order.side.toUpperCase()}
                          </Badge>
                          <Badge variant="outline">{order.status}</Badge>
                        </div>
                        <div className="text-sm text-muted-foreground">
                          {order.quantity} shares {order.price && `@ ${formatCurrency(order.price)}`}
                        </div>
                      </div>
                      <div className="text-right text-sm text-muted-foreground">
                        {new Date(order.created_at).toLocaleString()}
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
              <CardTitle>System Alerts</CardTitle>
              <CardDescription>Recent system notifications and warnings</CardDescription>
            </CardHeader>
            <CardContent>
              {alerts.length === 0 ? (
                <div className="text-center py-8">
                  <CheckCircle className="w-12 h-12 text-green-600 mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">All systems operational</h3>
                  <p className="text-muted-foreground">No alerts at this time</p>
                </div>
              ) : (
                <div className="space-y-4">
                  {alerts.map((alert) => (
                    <div key={alert.id} className="flex items-start gap-3 p-4 border rounded-lg">
                      {getAlertIcon(alert.level)}
                      <div className="flex-1">
                        <p className="text-sm font-medium">{alert.message}</p>
                        <div className="flex items-center gap-2 mt-1">
                          <Badge variant="outline" className="text-xs">{alert.category}</Badge>
                          <span className="text-xs text-muted-foreground">
                            {new Date(alert.timestamp).toLocaleString()}
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
  )
}