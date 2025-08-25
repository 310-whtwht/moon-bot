'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ArrowUpDown, DollarSign, Clock, CheckCircle, XCircle, AlertCircle, Eye } from 'lucide-react'
import Link from 'next/link'

interface Order {
  id: string
  client_order_id: string
  strategy_id?: string
  symbol: string
  side: string
  order_type: string
  quantity: number
  price?: number
  stop_price?: number
  status: string
  filled_quantity: number
  avg_fill_price?: number
  commission: number
  broker_order_id?: string
  error_message?: string
  created_at: string
  updated_at: string
}

interface Trade {
  id: string
  order_id: string
  strategy_id?: string
  symbol: string
  side: string
  quantity: number
  price: number
  commission: number
  broker_trade_id?: string
  trade_time: string
  created_at: string
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

export default function OrdersPage() {
  const [orders, setOrders] = useState<Order[]>([])
  const [trades, setTrades] = useState<Trade[]>([])
  const [strategies, setStrategies] = useState<Map<string, Strategy>>(new Map())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [sideFilter, setSideFilter] = useState('all')

  useEffect(() => {
    fetchOrders()
    fetchTrades()
    fetchStrategies()
  }, [])

  const fetchOrders = async () => {
    try {
      setLoading(true)
      const response = await fetch('/api/v1/orders')
      if (!response.ok) {
        throw new Error('Failed to fetch orders')
      }
      const data = await response.json()
      setOrders(data.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setLoading(false)
    }
  }

  const fetchTrades = async () => {
    try {
      const response = await fetch('/api/v1/trades')
      if (response.ok) {
        const data = await response.json()
        setTrades(data.data || [])
      }
    } catch (err) {
      console.error('Failed to fetch trades:', err)
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
      case 'filled':
        return <CheckCircle className="w-4 h-4 text-green-600" />
      case 'partial':
        return <Clock className="w-4 h-4 text-yellow-600" />
      case 'pending':
      case 'submitted':
        return <Clock className="w-4 h-4 text-blue-600" />
      case 'cancelled':
        return <XCircle className="w-4 h-4 text-gray-600" />
      case 'rejected':
        return <AlertCircle className="w-4 h-4 text-red-600" />
      default:
        return <Clock className="w-4 h-4 text-yellow-600" />
    }
  }

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'filled':
        return 'bg-green-100 text-green-800'
      case 'partial':
        return 'bg-yellow-100 text-yellow-800'
      case 'pending':
      case 'submitted':
        return 'bg-blue-100 text-blue-800'
      case 'cancelled':
        return 'bg-gray-100 text-gray-800'
      case 'rejected':
        return 'bg-red-100 text-red-800'
      default:
        return 'bg-yellow-100 text-yellow-800'
    }
  }

  const getSideColor = (side: string) => {
    return side === 'buy' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
  }

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(num)
  }

  const formatCurrency = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(num)
  }

  const filteredOrders = orders.filter(order => {
    const matchesSearch = order.symbol.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         order.client_order_id.toLowerCase().includes(searchTerm.toLowerCase())
    const matchesStatus = statusFilter === 'all' || order.status === statusFilter
    const matchesSide = sideFilter === 'all' || order.side === sideFilter
    return matchesSearch && matchesStatus && matchesSide
  })

  const filteredTrades = trades.filter(trade => {
    return trade.symbol.toLowerCase().includes(searchTerm.toLowerCase())
  })

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading orders...</div>
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
          <h1 className="text-3xl font-bold">Orders & Trades</h1>
          <p className="text-muted-foreground">Monitor your trading activity</p>
        </div>
      </div>

      <Tabs defaultValue="orders" className="w-full">
        <TabsList>
          <TabsTrigger value="orders">Orders</TabsTrigger>
          <TabsTrigger value="trades">Trades</TabsTrigger>
        </TabsList>

        <TabsContent value="orders" className="mt-6">
          {/* Filters */}
          <div className="mb-6 flex gap-4">
            <div className="flex-1">
              <Input
                placeholder="Search orders by symbol or order ID..."
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
              <option value="submitted">Submitted</option>
              <option value="partial">Partial</option>
              <option value="filled">Filled</option>
              <option value="cancelled">Cancelled</option>
              <option value="rejected">Rejected</option>
            </select>
            <select
              value={sideFilter}
              onChange={(e) => setSideFilter(e.target.value)}
              className="px-3 py-2 border border-input rounded-md bg-background"
            >
              <option value="all">All Sides</option>
              <option value="buy">Buy</option>
              <option value="sell">Sell</option>
            </select>
          </div>

          {filteredOrders.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center h-64">
                <ArrowUpDown className="w-12 h-12 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold mb-2">No orders found</h3>
                <p className="text-muted-foreground">
                  {searchTerm || statusFilter !== 'all' || sideFilter !== 'all'
                    ? 'Try adjusting your search or filters'
                    : 'Orders will appear here when you place trades'
                  }
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {filteredOrders.map((order) => {
                const strategy = order.strategy_id ? strategies.get(order.strategy_id) : null

                return (
                  <Card key={order.id} className="hover:shadow-lg transition-shadow">
                    <CardHeader>
                      <div className="flex justify-between items-start">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-2">
                            <CardTitle className="text-lg">{order.symbol}</CardTitle>
                            <Badge className={getSideColor(order.side)}>
                              {order.side.toUpperCase()}
                            </Badge>
                            <Badge variant="outline">
                              {order.order_type}
                            </Badge>
                          </div>
                          <CardDescription>
                            {strategy?.name && `Strategy: ${strategy.name}`}
                            {!strategy?.name && `Order ID: ${order.client_order_id}`}
                          </CardDescription>
                        </div>
                        <div className="flex items-center gap-2">
                          {getStatusIcon(order.status)}
                          <Badge className={getStatusColor(order.status)}>
                            {order.status}
                          </Badge>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                        <div>
                          <span className="font-medium text-muted-foreground">Quantity:</span>
                          <div className="font-bold">{order.quantity}</div>
                        </div>
                        <div>
                          <span className="font-medium text-muted-foreground">Filled:</span>
                          <div className="font-bold">{order.filled_quantity}</div>
                        </div>
                        {order.price && (
                          <div>
                            <span className="font-medium text-muted-foreground">Price:</span>
                            <div className="font-bold">{formatCurrency(order.price)}</div>
                          </div>
                        )}
                        {order.avg_fill_price && (
                          <div>
                            <span className="font-medium text-muted-foreground">Avg Fill:</span>
                            <div className="font-bold">{formatCurrency(order.avg_fill_price)}</div>
                          </div>
                        )}
                      </div>
                      <div className="flex justify-between items-center mt-4 pt-4 border-t">
                        <div className="text-sm text-muted-foreground">
                          Created: {new Date(order.created_at).toLocaleDateString()}
                        </div>
                        <Link href={`/orders/${order.id}`}>
                          <Button variant="outline" size="sm">
                            <Eye className="w-4 h-4 mr-1" />
                            View Details
                          </Button>
                        </Link>
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </TabsContent>

        <TabsContent value="trades" className="mt-6">
          {/* Filters */}
          <div className="mb-6">
            <Input
              placeholder="Search trades by symbol..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>

          {filteredTrades.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center h-64">
                <DollarSign className="w-12 h-12 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold mb-2">No trades found</h3>
                <p className="text-muted-foreground">
                  {searchTerm
                    ? 'Try adjusting your search'
                    : 'Trades will appear here when orders are executed'
                  }
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {filteredTrades.map((trade) => {
                const strategy = trade.strategy_id ? strategies.get(trade.strategy_id) : null

                return (
                  <Card key={trade.id} className="hover:shadow-lg transition-shadow">
                    <CardHeader>
                      <div className="flex justify-between items-start">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-2">
                            <CardTitle className="text-lg">{trade.symbol}</CardTitle>
                            <Badge className={getSideColor(trade.side)}>
                              {trade.side.toUpperCase()}
                            </Badge>
                          </div>
                          <CardDescription>
                            {strategy?.name && `Strategy: ${strategy.name}`}
                            {!strategy?.name && `Trade ID: ${trade.id}`}
                          </CardDescription>
                        </div>
                        <div className="text-right">
                          <div className="text-lg font-bold">{formatCurrency(trade.price)}</div>
                          <div className="text-sm text-muted-foreground">{trade.quantity} shares</div>
                        </div>
                      </div>
                    </CardHeader>
                    <CardContent>
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                        <div>
                          <span className="font-medium text-muted-foreground">Quantity:</span>
                          <div className="font-bold">{trade.quantity}</div>
                        </div>
                        <div>
                          <span className="font-medium text-muted-foreground">Price:</span>
                          <div className="font-bold">{formatCurrency(trade.price)}</div>
                        </div>
                        <div>
                          <span className="font-medium text-muted-foreground">Commission:</span>
                          <div className="font-bold">{formatCurrency(trade.commission)}</div>
                        </div>
                        <div>
                          <span className="font-medium text-muted-foreground">Total:</span>
                          <div className="font-bold">{formatCurrency(trade.quantity * trade.price)}</div>
                        </div>
                      </div>
                      <div className="mt-4 pt-4 border-t text-sm text-muted-foreground">
                        Executed: {new Date(trade.trade_time).toLocaleString()}
                      </div>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}