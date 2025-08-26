'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Search, Download, Eye, BarChart3 } from 'lucide-react'
import { DatePicker } from '@/components/ui/date-picker'

interface TradeTrace {
  id: string
  strategy_id: string
  symbol: string
  side: 'buy' | 'sell'
  quantity: number
  price: number
  timestamp: string
  order_id: string
  trade_id: string
  parent_id?: string
  trace_id: string
  metadata?: Record<string, unknown>
}

interface TraceChain {
  trace_id: string
  chain: TradeTrace[]
  length: number
}

interface TraceStatistics {
  total_trades: number
  total_volume: number
  total_value: number
  buy_count: number
  sell_count: number
  avg_price: number
  max_price: number
  min_price: number
}

export default function TraceVisualizer() {
  const [traces, setTraces] = useState<TradeTrace[]>([])
  const [traceChain, setTraceChain] = useState<TraceChain | null>(null)
  const [statistics, setStatistics] = useState<TraceStatistics | null>(null)
  const [loading, setLoading] = useState(false)
  const [searchFilters, setSearchFilters] = useState({
    strategy_id: '',
    symbol: '',
    side: '',
    start_time: '',
    end_time: '',
  })

  const [viewMode, setViewMode] = useState<'list' | 'chain' | 'stats'>('list')

  // トレース検索
  const searchTraces = useCallback(async () => {
    setLoading(true)
    try {
      const response = await fetch('/api/audit/traces/search', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(searchFilters),
      })
      
      if (response.ok) {
        const data = await response.json()
        setTraces(data.traces)
      }
    } catch (error) {
      console.error('Failed to search traces:', error)
    } finally {
      setLoading(false)
    }
  }, [searchFilters])

  // トレース鎖取得
  const getTraceChain = async (traceId: string) => {
    setLoading(true)
    try {
      const response = await fetch(`/api/audit/traces/${traceId}/chain`)
      
      if (response.ok) {
        const data = await response.json()
        setTraceChain(data)
        setViewMode('chain')
      }
    } catch (error) {
      console.error('Failed to get trace chain:', error)
    } finally {
      setLoading(false)
    }
  }

  // 統計情報取得
  const getStatistics = async (strategyId: string) => {
    setLoading(true)
    try {
      const params = new URLSearchParams({
        start_time: searchFilters.start_time,
        end_time: searchFilters.end_time,
      })
      
      const response = await fetch(`/api/audit/traces/strategy/${strategyId}/statistics?${params}`)
      
      if (response.ok) {
        const data = await response.json()
        setStatistics(data.statistics)
        setViewMode('stats')
      }
    } catch (error) {
      console.error('Failed to get statistics:', error)
    } finally {
      setLoading(false)
    }
  }

  // エクスポート
  const exportTraces = async (format: 'json' | 'csv') => {
    try {
      const response = await fetch(`/api/audit/traces/export?format=${format}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(searchFilters),
      })
      
      if (response.ok) {
        const blob = await response.blob()
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `traces.${format}`
        document.body.appendChild(a)
        a.click()
        window.URL.revokeObjectURL(url)
        document.body.removeChild(a)
      }
    } catch (error) {
      console.error('Failed to export traces:', error)
    }
  }

  useEffect(() => {
    searchTraces()
  }, [searchTraces])

  const renderTraceList = () => (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-semibold">トレース一覧</h3>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportTraces('json')}
          >
            <Download className="w-4 h-4 mr-2" />
            JSON
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportTraces('csv')}
          >
            <Download className="w-4 h-4 mr-2" />
            CSV
          </Button>
        </div>
      </div>
      
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>トレースID</TableHead>
            <TableHead>戦略ID</TableHead>
            <TableHead>銘柄</TableHead>
            <TableHead>サイド</TableHead>
            <TableHead>数量</TableHead>
            <TableHead>価格</TableHead>
            <TableHead>時刻</TableHead>
            <TableHead>操作</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {traces.map((trace) => (
            <TableRow key={trace.id}>
              <TableCell className="font-mono text-sm">
                {trace.trace_id.slice(0, 8)}...
              </TableCell>
              <TableCell>{trace.strategy_id}</TableCell>
              <TableCell>{trace.symbol}</TableCell>
              <TableCell>
                <Badge variant={trace.side === 'buy' ? 'default' : 'secondary'}>
                  {trace.side}
                </Badge>
              </TableCell>
              <TableCell>{trace.quantity.toLocaleString()}</TableCell>
              <TableCell>${trace.price.toFixed(2)}</TableCell>
              <TableCell>
                {new Date(trace.timestamp).toLocaleString('ja-JP')}
              </TableCell>
              <TableCell>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => getTraceChain(trace.trace_id)}
                >
                  <Eye className="w-4 h-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )

  const renderTraceChain = () => (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-semibold">トレース鎖</h3>
        <Button
          variant="outline"
          onClick={() => setViewMode('list')}
        >
          一覧に戻る
        </Button>
      </div>
      
      {traceChain && (
        <div className="space-y-4">
          <div className="flex items-center gap-4">
            <Badge variant="outline">
              鎖の長さ: {traceChain.length}
            </Badge>
            <Badge variant="outline">
              トレースID: {traceChain.trace_id.slice(0, 8)}...
            </Badge>
          </div>
          
          <div className="space-y-2">
            {traceChain.chain.map((trace, index) => (
              <Card key={trace.id} className="border-l-4 border-blue-500">
                <CardContent className="p-4">
                  <div className="flex justify-between items-center">
                    <div className="flex items-center gap-4">
                      <Badge variant="outline">#{index + 1}</Badge>
                      <div>
                        <div className="font-semibold">{trace.symbol}</div>
                        <div className="text-sm text-gray-500">
                          {trace.strategy_id} - {trace.order_id}
                        </div>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="flex items-center gap-2">
                        <Badge variant={trace.side === 'buy' ? 'default' : 'secondary'}>
                          {trace.side}
                        </Badge>
                        <span className="font-semibold">
                          {trace.quantity.toLocaleString()} @ ${trace.price.toFixed(2)}
                        </span>
                      </div>
                      <div className="text-sm text-gray-500">
                        {new Date(trace.timestamp).toLocaleString('ja-JP')}
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  )

  const renderStatistics = () => (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h3 className="text-lg font-semibold">統計情報</h3>
        <Button
          variant="outline"
          onClick={() => setViewMode('list')}
        >
          一覧に戻る
        </Button>
      </div>
      
      {statistics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold">{statistics.total_trades}</div>
              <div className="text-sm text-gray-500">総取引数</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold">{statistics.total_volume.toLocaleString()}</div>
              <div className="text-sm text-gray-500">総数量</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold">${statistics.total_value.toLocaleString()}</div>
              <div className="text-sm text-gray-500">総取引額</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold">${statistics.avg_price.toFixed(2)}</div>
              <div className="text-sm text-gray-500">平均価格</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-green-600">{statistics.buy_count}</div>
              <div className="text-sm text-gray-500">買い注文数</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-red-600">{statistics.sell_count}</div>
              <div className="text-sm text-gray-500">売り注文数</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-green-600">${statistics.max_price.toFixed(2)}</div>
              <div className="text-sm text-gray-500">最高価格</div>
            </CardContent>
          </Card>
          
          <Card>
            <CardContent className="p-4">
              <div className="text-2xl font-bold text-red-600">${statistics.min_price.toFixed(2)}</div>
              <div className="text-sm text-gray-500">最低価格</div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  )

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BarChart3 className="w-5 h-5" />
            監査ログ・トレース可視化
          </CardTitle>
        </CardHeader>
        <CardContent>
          {/* 検索フィルター */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
            <div>
              <Label htmlFor="strategy_id">戦略ID</Label>
              <Input
                id="strategy_id"
                value={searchFilters.strategy_id}
                onChange={(e) => setSearchFilters(prev => ({ ...prev, strategy_id: e.target.value }))}
                placeholder="戦略IDを入力"
              />
            </div>
            
            <div>
              <Label htmlFor="symbol">銘柄</Label>
              <Input
                id="symbol"
                value={searchFilters.symbol}
                onChange={(e) => setSearchFilters(prev => ({ ...prev, symbol: e.target.value }))}
                placeholder="AAPL"
              />
            </div>
            
            <div>
              <Label htmlFor="side">サイド</Label>
              <Select
                value={searchFilters.side}
                onValueChange={(value) => setSearchFilters(prev => ({ ...prev, side: value }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="サイドを選択" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">すべて</SelectItem>
                  <SelectItem value="buy">買い</SelectItem>
                  <SelectItem value="sell">売り</SelectItem>
                </SelectContent>
              </Select>
            </div>
            
            <div>
              <Label>開始日時</Label>
              <DatePicker
                value={searchFilters.start_time}
                onChange={(date) => setSearchFilters(prev => ({ 
                  ...prev, 
                  start_time: date ? date.toISOString() : '' 
                }))}
              />
            </div>
            
            <div>
              <Label>終了日時</Label>
              <DatePicker
                value={searchFilters.end_time}
                onChange={(date) => setSearchFilters(prev => ({ 
                  ...prev, 
                  end_time: date ? date.toISOString() : '' 
                }))}
              />
            </div>
            
            <div className="flex items-end">
              <Button onClick={searchTraces} disabled={loading} className="w-full">
                <Search className="w-4 h-4 mr-2" />
                検索
              </Button>
            </div>
          </div>

          {/* 表示モード切り替え */}
          <div className="flex gap-2 mb-4">
            <Button
              variant={viewMode === 'list' ? 'default' : 'outline'}
              onClick={() => setViewMode('list')}
            >
              一覧
            </Button>
            <Button
              variant={viewMode === 'stats' ? 'default' : 'outline'}
              onClick={() => {
                if (searchFilters.strategy_id) {
                  getStatistics(searchFilters.strategy_id)
                }
              }}
            >
              統計
            </Button>
          </div>

          {/* コンテンツ表示 */}
          {loading ? (
            <div className="flex justify-center items-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900"></div>
            </div>
          ) : (
            <>
              {viewMode === 'list' && renderTraceList()}
              {viewMode === 'chain' && renderTraceChain()}
              {viewMode === 'stats' && renderStatistics()}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}