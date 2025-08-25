'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Play, BarChart3, Clock, CheckCircle, XCircle, AlertCircle } from 'lucide-react'
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

export default function BacktestsPage() {
  const [backtests, setBacktests] = useState<Backtest[]>([])
  const [strategies, setStrategies] = useState<Map<string, Strategy>>(new Map())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')

  useEffect(() => {
    fetchBacktests()
    fetchStrategies()
  }, [])

  const fetchBacktests = async () => {
    try {
      setLoading(true)
      const response = await fetch('/api/v1/backtests')
      if (!response.ok) {
        throw new Error('Failed to fetch backtests')
      }
      const data = await response.json()
      setBacktests(data.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setLoading(false)
    }
  }

  const fetchStrategies = async () => {
    try {
      const response = await fetch('/api/v1/strategies?limit=100')
      if (response.ok) {
        const data = await response.json()
        const strategyMap = new Map()
        data.data.forEach((strategy: Strategy) => {
          strategyMap.set(strategy.id, strategy)
        })
        setStrategies(strategyMap)
      }
    } catch (err) {
      console.error('Failed to fetch strategies:', err)
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-4 h-4 text-green-600" />
      case 'running':
        return <Clock className="w-4 h-4 text-blue-600" />
      case 'failed':
        return <XCircle className="w-4 h-4 text-red-600" />
      case 'cancelled':
        return <AlertCircle className="w-4 h-4 text-gray-600" />
      default:
        return <Clock className="w-4 h-4 text-yellow-600" />
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

  const filteredBacktests = backtests.filter(backtest => {
    const matchesSearch = backtest.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         backtest.symbols.some(symbol => symbol.toLowerCase().includes(searchTerm.toLowerCase()))
    const matchesStatus = statusFilter === 'all' || backtest.status === statusFilter
    return matchesSearch && matchesStatus
  })

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading backtests...</div>
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
          <h1 className="text-3xl font-bold">Backtests</h1>
          <p className="text-muted-foreground">View and manage your strategy backtests</p>
        </div>
        <Link href="/strategies">
          <Button>
            <Play className="w-4 h-4 mr-2" />
            New Backtest
          </Button>
        </Link>
      </div>

      {/* Filters */}
      <div className="mb-6 flex gap-4">
        <div className="flex-1">
          <Input
            placeholder="Search backtests..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-3 py-2 border border-input rounded-md bg-background"
        >
          <option value="all">All Status</option>
          <option value="pending">Pending</option>
          <option value="running">Running</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="cancelled">Cancelled</option>
        </select>
      </div>

      {filteredBacktests.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center h-64">
            <BarChart3 className="w-12 h-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No backtests found</h3>
            <p className="text-muted-foreground mb-4">
              {searchTerm || statusFilter !== 'all' 
                ? 'Try adjusting your search or filters'
                : 'Run your first backtest to see results here'
              }
            </p>
            {!searchTerm && statusFilter === 'all' && (
              <Link href="/strategies">
                <Button>
                  <Play className="w-4 h-4 mr-2" />
                  Run Backtest
                </Button>
              </Link>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {filteredBacktests.map((backtest) => {
            const strategy = strategies.get(backtest.strategy_id)
            const results = backtest.results ? JSON.parse(backtest.results) : null

            return (
              <Card key={backtest.id} className="hover:shadow-lg transition-shadow">
                <CardHeader>
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <CardTitle className="text-lg">{backtest.name}</CardTitle>
                      <CardDescription className="mt-1">
                        {strategy?.name && `Strategy: ${strategy.name}`}
                      </CardDescription>
                    </div>
                    <div className="flex items-center gap-1">
                      {getStatusIcon(backtest.status)}
                      <Badge className={getStatusColor(backtest.status)}>
                        {backtest.status}
                      </Badge>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    <div className="text-sm text-muted-foreground">
                      <span className="font-medium">Symbols:</span> {backtest.symbols.join(', ')}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      <span className="font-medium">Date Range:</span>{' '}
                      {new Date(backtest.start_date).toLocaleDateString()} - {new Date(backtest.end_date).toLocaleDateString()}
                    </div>
                    <div className="text-sm text-muted-foreground">
                      <span className="font-medium">Created:</span>{' '}
                      {new Date(backtest.created_at).toLocaleDateString()}
                    </div>
                    
                    {backtest.status === 'running' && (
                      <div>
                        <div className="text-sm text-muted-foreground mb-1">Progress</div>
                        <div className="w-full bg-gray-200 rounded-full h-2">
                          <div 
                            className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                            style={{ width: `${backtest.progress}%` }}
                          ></div>
                        </div>
                        <div className="text-xs text-muted-foreground mt-1">{backtest.progress}%</div>
                      </div>
                    )}

                    {backtest.status === 'completed' && results && (
                      <div className="grid grid-cols-2 gap-2 text-sm">
                        <div>
                          <span className="font-medium">Return:</span>
                          <div className={`font-bold ${results.total_return >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                            {formatPercentage(results.total_return)}
                          </div>
                        </div>
                        <div>
                          <span className="font-medium">Sharpe:</span>
                          <div className="font-bold">{formatNumber(results.sharpe_ratio)}</div>
                        </div>
                        <div>
                          <span className="font-medium">Max DD:</span>
                          <div className="font-bold text-red-600">{formatPercentage(results.max_drawdown)}</div>
                        </div>
                        <div>
                          <span className="font-medium">Win Rate:</span>
                          <div className="font-bold">{formatNumber(results.win_rate)}%</div>
                        </div>
                      </div>
                    )}

                    <div className="flex gap-2 pt-2">
                      <Link href={`/backtests/${backtest.id}`} className="flex-1">
                        <Button variant="outline" size="sm" className="w-full">
                          <BarChart3 className="w-4 h-4 mr-1" />
                          View Details
                        </Button>
                      </Link>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}