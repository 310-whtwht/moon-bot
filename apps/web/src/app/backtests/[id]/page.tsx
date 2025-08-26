'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ArrowLeft, Play, Download, Trash2, BarChart3, TrendingUp, TrendingDown } from 'lucide-react'
import Link from 'next/link'

interface Backtest {
  id: string
  name: string
  strategy_id: string
  symbols: string[]
  start_date: string
  end_date: string
  parameters: any
  status: string
  progress: number
  results: any
  error?: string
  created_at: string
  updated_at: string
  completed_at?: string
}

interface Strategy {
  id: string
  name: string
  description?: string
  author: string
  is_public: boolean
  created_at: string
  updated_at: string
}

export default function BacktestDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [backtest, setBacktest] = useState<Backtest | null>(null)
  const [strategy, setStrategy] = useState<Strategy | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const backtestId = params.id as string

  useEffect(() => {
    if (backtestId) {
      fetchBacktest()
    }
  }, [backtestId])

  const fetchBacktest = async () => {
    try {
      const response = await fetch(`/api/v1/backtests/${backtestId}`)
      if (!response.ok) {
        throw new Error('Failed to fetch backtest')
      }
      const data = await response.json()
      setBacktest(data.data)
      
      // Fetch strategy information
      if (data.data.strategy_id) {
        fetchStrategy(data.data.strategy_id)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setLoading(false)
    }
  }

  const fetchStrategy = async (strategyId: string) => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}`)
      if (response.ok) {
        const data = await response.json()
        setStrategy(data.data)
      }
    } catch (err) {
      console.error('Failed to fetch strategy:', err)
    }
  }

  const handleDeleteBacktest = async () => {
    if (!confirm('Are you sure you want to delete this backtest? This action cannot be undone.')) {
      return
    }

    try {
      const response = await fetch(`/api/v1/backtests/${backtestId}`, {
        method: 'DELETE',
      })
      if (!response.ok) {
        throw new Error('Failed to delete backtest')
      }
      router.push('/backtests')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete backtest')
    }
  }

  const handleCancelBacktest = async () => {
    if (!confirm('Are you sure you want to cancel this backtest?')) {
      return
    }

    try {
      const response = await fetch(`/api/v1/backtests/${backtestId}/cancel`, {
        method: 'POST',
      })
      if (!response.ok) {
        throw new Error('Failed to cancel backtest')
      }
      await fetchBacktest() // Refresh data
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel backtest')
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800'
      case 'running':
        return 'bg-blue-100 text-blue-800'
      case 'failed':
        return 'bg-red-100 text-red-800'
      case 'cancelled':
        return 'bg-gray-100 text-gray-800'
      default:
        return 'bg-yellow-100 text-yellow-800'
    }
  }

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(num)
  }

  const formatPercentage = (num: number) => {
    return `${num >= 0 ? '+' : ''}${formatNumber(num)}%`
  }

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading backtest...</div>
        </div>
      </div>
    )
  }

  if (error || !backtest) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">Error: {error || 'Backtest not found'}</div>
        </div>
      </div>
    )
  }

  const results = backtest.results ? JSON.parse(backtest.results) : null

  return (
    <div className="container mx-auto p-6">
      <div className="mb-6">
        <Link href="/backtests" className="inline-flex items-center text-muted-foreground hover:text-foreground">
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Backtests
        </Link>
      </div>

      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-3xl font-bold">{backtest.name}</h1>
          <p className="text-muted-foreground mt-2">
            {strategy?.name && `Strategy: ${strategy.name}`}
          </p>
        </div>
        <div className="flex gap-2">
          {backtest.status === 'running' && (
            <Button variant="outline" onClick={handleCancelBacktest}>
              Cancel
            </Button>
          )}
          <Button variant="outline" onClick={handleDeleteBacktest} className="text-red-600 hover:text-red-700">
            <Trash2 className="w-4 h-4 mr-2" />
            Delete
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Backtest Info */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle>Backtest Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">Status</label>
                <div className="mt-1">
                  <Badge className={getStatusColor(backtest.status)}>
                    {backtest.status}
                  </Badge>
                </div>
              </div>
              {backtest.status === 'running' && (
                <div>
                  <label className="text-sm font-medium text-muted-foreground">Progress</label>
                  <div className="mt-1">
                    <div className="w-full bg-gray-200 rounded-full h-2">
                      <div 
                        className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                        style={{ width: `${backtest.progress}%` }}
                      ></div>
                    </div>
                    <p className="text-sm text-muted-foreground mt-1">{backtest.progress}%</p>
                  </div>
                </div>
              )}
              <div>
                <label className="text-sm font-medium text-muted-foreground">Symbols</label>
                <p className="mt-1">{backtest.symbols.join(', ')}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">Date Range</label>
                <p className="mt-1">
                  {new Date(backtest.start_date).toLocaleDateString()} - {new Date(backtest.end_date).toLocaleDateString()}
                </p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">Created</label>
                <p className="mt-1">{new Date(backtest.created_at).toLocaleDateString()}</p>
              </div>
              {backtest.completed_at && (
                <div>
                  <label className="text-sm font-medium text-muted-foreground">Completed</label>
                  <p className="mt-1">{new Date(backtest.completed_at).toLocaleDateString()}</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Main Content */}
        <div className="lg:col-span-2">
          <Tabs defaultValue="results" className="w-full">
            <TabsList>
              <TabsTrigger value="results">Results</TabsTrigger>
              <TabsTrigger value="trades">Trades</TabsTrigger>
              <TabsTrigger value="parameters">Parameters</TabsTrigger>
            </TabsList>

            <TabsContent value="results" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Performance Results</CardTitle>
                  <CardDescription>
                    {backtest.status === 'completed' ? 'Backtest completed successfully' : 'Results will appear when backtest completes'}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {backtest.status === 'completed' && results ? (
                    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                      <div className="p-4 border rounded-lg">
                        <div className="flex items-center gap-2 mb-2">
                          <TrendingUp className="w-4 h-4 text-green-600" />
                          <h4 className="font-medium">Total Return</h4>
                        </div>
                        <p className={`text-2xl font-bold ${results.total_return >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                          {formatPercentage(results.total_return)}
                        </p>
                      </div>
                      <div className="p-4 border rounded-lg">
                        <div className="flex items-center gap-2 mb-2">
                          <BarChart3 className="w-4 h-4 text-blue-600" />
                          <h4 className="font-medium">Sharpe Ratio</h4>
                        </div>
                        <p className="text-2xl font-bold">{formatNumber(results.sharpe_ratio)}</p>
                      </div>
                      <div className="p-4 border rounded-lg">
                        <div className="flex items-center gap-2 mb-2">
                          <TrendingDown className="w-4 h-4 text-red-600" />
                          <h4 className="font-medium">Max Drawdown</h4>
                        </div>
                        <p className="text-2xl font-bold text-red-600">
                          {formatPercentage(results.max_drawdown)}
                        </p>
                      </div>
                      <div className="p-4 border rounded-lg">
                        <h4 className="font-medium mb-2">Win Rate</h4>
                        <p className="text-2xl font-bold">{formatNumber(results.win_rate)}%</p>
                      </div>
                      <div className="p-4 border rounded-lg">
                        <h4 className="font-medium mb-2">Total Trades</h4>
                        <p className="text-2xl font-bold">{results.total_trades}</p>
                      </div>
                      <div className="p-4 border rounded-lg">
                        <h4 className="font-medium mb-2">Profit Factor</h4>
                        <p className="text-2xl font-bold">{formatNumber(results.profit_factor)}</p>
                      </div>
                    </div>
                  ) : backtest.status === 'failed' ? (
                    <div className="text-center py-8">
                      <div className="text-red-500 mb-4">
                        <TrendingDown className="w-12 h-12 mx-auto" />
                      </div>
                      <h3 className="text-lg font-semibold mb-2">Backtest Failed</h3>
                      <p className="text-muted-foreground mb-4">{backtest.error}</p>
                    </div>
                  ) : (
                    <div className="text-center py-8">
                      <div className="text-blue-500 mb-4">
                        <Play className="w-12 h-12 mx-auto animate-pulse" />
                      </div>
                      <h3 className="text-lg font-semibold mb-2">Backtest in Progress</h3>
                      <p className="text-muted-foreground">Results will be available when the backtest completes.</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="trades" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Trade History</CardTitle>
                  <CardDescription>
                    Detailed list of all trades executed during the backtest
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {backtest.status === 'completed' && results?.trades ? (
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b">
                            <th className="text-left py-2">Date</th>
                            <th className="text-left py-2">Symbol</th>
                            <th className="text-left py-2">Side</th>
                            <th className="text-left py-2">Quantity</th>
                            <th className="text-left py-2">Price</th>
                            <th className="text-left py-2">PnL</th>
                          </tr>
                        </thead>
                        <tbody>
                          {results.trades.map((trade: any, index: number) => (
                            <tr key={index} className="border-b">
                              <td className="py-2">{new Date(trade.date).toLocaleDateString()}</td>
                              <td className="py-2">{trade.symbol}</td>
                              <td className={`py-2 ${trade.side === 'BUY' ? 'text-green-600' : 'text-red-600'}`}>
                                {trade.side}
                              </td>
                              <td className="py-2">{trade.quantity}</td>
                              <td className="py-2">${trade.price}</td>
                              <td className={`py-2 ${trade.pnl >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                                {formatNumber(trade.pnl)}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  ) : (
                    <div className="text-center py-8">
                      <p className="text-muted-foreground">Trade history will be available when the backtest completes.</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="parameters" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Backtest Parameters</CardTitle>
                  <CardDescription>
                    Configuration used for this backtest
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="space-y-4">
                    <div>
                      <label className="text-sm font-medium text-muted-foreground">Strategy Parameters</label>
                      <pre className="mt-1 p-3 bg-muted rounded-lg overflow-x-auto text-sm">
                        <code>
                          {backtest.parameters ? JSON.stringify(backtest.parameters, null, 2) : 'No parameters'}
                        </code>
                      </pre>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}