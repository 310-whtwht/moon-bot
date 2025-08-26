'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams } from 'next/navigation';
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
  ArrowLeft,
  XCircle,
  CheckCircle,
  Clock,
  AlertCircle,
  DollarSign,
} from 'lucide-react';
import Link from 'next/link';

interface Order {
  id: string;
  client_order_id: string;
  strategy_id?: string;
  symbol: string;
  side: string;
  order_type: string;
  quantity: number;
  price?: number;
  stop_price?: number;
  status: string;
  filled_quantity: number;
  avg_fill_price?: number;
  commission: number;
  broker_order_id?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

interface Trade {
  id: string;
  order_id: string;
  strategy_id?: string;
  symbol: string;
  side: string;
  quantity: number;
  price: number;
  commission: number;
  broker_trade_id?: string;
  trade_time: string;
  created_at: string;
}

interface Strategy {
  id: string;
  name: string;
  description?: string;
  author: string;
  is_public: boolean;
  created_at: string;
  updated_at: string;
}

export default function OrderDetailPage() {
  const params = useParams();

  const [order, setOrder] = useState<Order | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [strategy, setStrategy] = useState<Strategy | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const orderId = params.id as string;

  const fetchOrder = useCallback(async () => {
    try {
      const response = await fetch(`/api/v1/orders/${orderId}`);
      if (!response.ok) {
        throw new Error('Failed to fetch order');
      }
      const data = await response.json();
      setOrder(data.data.order);
      setTrades(data.data.trades || []);

      // Fetch strategy information
      if (data.data.order.strategy_id) {
        fetchStrategy(data.data.order.strategy_id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  }, [orderId]);

  useEffect(() => {
    if (orderId) {
      fetchOrder();
    }
  }, [orderId, fetchOrder]);

  const fetchStrategy = async (strategyId: string) => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}`);
      if (response.ok) {
        const data = await response.json();
        setStrategy(data.data);
      }
    } catch (err) {
      console.error('Failed to fetch strategy:', err);
    }
  };

  const handleCancelOrder = async () => {
    if (!window.confirm('Are you sure you want to cancel this order?')) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/orders/${orderId}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error('Failed to cancel order');
      }
      await fetchOrder(); // Refresh data
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to cancel order');
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'filled':
        return <CheckCircle className="w-4 h-4 text-green-600" />;
      case 'partial':
        return <Clock className="w-4 h-4 text-yellow-600" />;
      case 'pending':
      case 'submitted':
        return <Clock className="w-4 h-4 text-blue-600" />;
      case 'cancelled':
        return <XCircle className="w-4 h-4 text-gray-600" />;
      case 'rejected':
        return <AlertCircle className="w-4 h-4 text-red-600" />;
      default:
        return <Clock className="w-4 h-4 text-yellow-600" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'filled':
        return 'bg-green-100 text-green-800';
      case 'partial':
        return 'bg-yellow-100 text-yellow-800';
      case 'pending':
      case 'submitted':
        return 'bg-blue-100 text-blue-800';
      case 'cancelled':
        return 'bg-gray-100 text-gray-800';
      case 'rejected':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-yellow-100 text-yellow-800';
    }
  };

  const getSideColor = (side: string) => {
    return side === 'buy'
      ? 'bg-green-100 text-green-800'
      : 'bg-red-100 text-red-800';
  };

  const formatCurrency = (num: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(num);
  };

  const canCancel =
    order && ['pending', 'submitted', 'partial'].includes(order.status);

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading order...</div>
        </div>
      </div>
    );
  }

  if (error || !order) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">
            Error: {error || 'Order not found'}
          </div>
        </div>
      </div>
    );
  }

  const totalValue = order.quantity * (order.price || 0);
  const filledValue = order.filled_quantity * (order.avg_fill_price || 0);

  return (
    <div className="container mx-auto p-6">
      <div className="mb-6">
        <Link
          href="/orders"
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Orders
        </Link>
      </div>

      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-3xl font-bold">{order.symbol}</h1>
          <p className="text-muted-foreground mt-2">
            {strategy?.name && `Strategy: ${strategy.name}`}
            {!strategy?.name && `Order ID: ${order.client_order_id}`}
          </p>
        </div>
        <div className="flex gap-2">
          {canCancel && (
            <Button
              variant="outline"
              onClick={handleCancelOrder}
              className="text-red-600 hover:text-red-700"
            >
              <XCircle className="w-4 h-4 mr-2" />
              Cancel Order
            </Button>
          )}
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Order Info */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle>Order Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Status
                </label>
                <div className="mt-1 flex items-center gap-2">
                  {getStatusIcon(order.status)}
                  <Badge className={getStatusColor(order.status)}>
                    {order.status}
                  </Badge>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Side
                </label>
                <div className="mt-1">
                  <Badge className={getSideColor(order.side)}>
                    {order.side.toUpperCase()}
                  </Badge>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Order Type
                </label>
                <p className="mt-1">{order.order_type}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Symbol
                </label>
                <p className="mt-1">{order.symbol}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Quantity
                </label>
                <p className="mt-1">{order.quantity}</p>
              </div>
              {order.price && (
                <div>
                  <label className="text-sm font-medium text-muted-foreground">
                    Price
                  </label>
                  <p className="mt-1">{formatCurrency(order.price)}</p>
                </div>
              )}
              {order.stop_price && (
                <div>
                  <label className="text-sm font-medium text-muted-foreground">
                    Stop Price
                  </label>
                  <p className="mt-1">{formatCurrency(order.stop_price)}</p>
                </div>
              )}
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Created
                </label>
                <p className="mt-1">
                  {new Date(order.created_at).toLocaleString()}
                </p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Last Updated
                </label>
                <p className="mt-1">
                  {new Date(order.updated_at).toLocaleString()}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Main Content */}
        <div className="lg:col-span-2">
          <Tabs defaultValue="execution" className="w-full">
            <TabsList>
              <TabsTrigger value="execution">Execution</TabsTrigger>
              <TabsTrigger value="trades">Trades</TabsTrigger>
            </TabsList>

            <TabsContent value="execution" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Execution Details</CardTitle>
                  <CardDescription>
                    Order execution status and fill information
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-6 md:grid-cols-2">
                    <div className="space-y-4">
                      <div className="p-4 border rounded-lg">
                        <h4 className="font-medium mb-2">Fill Progress</h4>
                        <div className="space-y-2">
                          <div className="flex justify-between">
                            <span className="text-sm text-muted-foreground">
                              Filled Quantity:
                            </span>
                            <span className="font-medium">
                              {order.filled_quantity}
                            </span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-sm text-muted-foreground">
                              Remaining:
                            </span>
                            <span className="font-medium">
                              {order.quantity - order.filled_quantity}
                            </span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-2">
                            <div
                              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                              style={{
                                width: `${(order.filled_quantity / order.quantity) * 100}%`,
                              }}
                            ></div>
                          </div>
                          <div className="text-xs text-muted-foreground text-center">
                            {(
                              (order.filled_quantity / order.quantity) *
                              100
                            ).toFixed(1)}
                            % filled
                          </div>
                        </div>
                      </div>

                      {order.avg_fill_price && (
                        <div className="p-4 border rounded-lg">
                          <h4 className="font-medium mb-2">
                            Average Fill Price
                          </h4>
                          <div className="text-2xl font-bold">
                            {formatCurrency(order.avg_fill_price)}
                          </div>
                        </div>
                      )}
                    </div>

                    <div className="space-y-4">
                      <div className="p-4 border rounded-lg">
                        <h4 className="font-medium mb-2">Order Value</h4>
                        <div className="space-y-2">
                          <div className="flex justify-between">
                            <span className="text-sm text-muted-foreground">
                              Total Value:
                            </span>
                            <span className="font-medium">
                              {formatCurrency(totalValue)}
                            </span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-sm text-muted-foreground">
                              Filled Value:
                            </span>
                            <span className="font-medium">
                              {formatCurrency(filledValue)}
                            </span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-sm text-muted-foreground">
                              Commission:
                            </span>
                            <span className="font-medium">
                              {formatCurrency(order.commission)}
                            </span>
                          </div>
                        </div>
                      </div>

                      {order.broker_order_id && (
                        <div className="p-4 border rounded-lg">
                          <h4 className="font-medium mb-2">
                            Broker Information
                          </h4>
                          <div className="space-y-2">
                            <div>
                              <span className="text-sm text-muted-foreground">
                                Broker Order ID:
                              </span>
                              <p className="font-mono text-sm">
                                {order.broker_order_id}
                              </p>
                            </div>
                          </div>
                        </div>
                      )}

                      {order.error_message && (
                        <div className="p-4 border border-red-200 rounded-lg bg-red-50">
                          <h4 className="font-medium mb-2 text-red-800">
                            Error Message
                          </h4>
                          <p className="text-sm text-red-700">
                            {order.error_message}
                          </p>
                        </div>
                      )}
                    </div>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="trades" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Trade History</CardTitle>
                  <CardDescription>
                    All trades executed for this order
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {trades.length === 0 ? (
                    <div className="text-center py-8">
                      <DollarSign className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                      <h3 className="text-lg font-semibold mb-2">
                        No trades yet
                      </h3>
                      <p className="text-muted-foreground">
                        Trades will appear here when the order is executed
                      </p>
                    </div>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b">
                            <th className="text-left py-2">Time</th>
                            <th className="text-left py-2">Quantity</th>
                            <th className="text-left py-2">Price</th>
                            <th className="text-left py-2">Commission</th>
                            <th className="text-left py-2">Total</th>
                          </tr>
                        </thead>
                        <tbody>
                          {trades.map(trade => (
                            <tr key={trade.id} className="border-b">
                              <td className="py-2">
                                {new Date(trade.trade_time).toLocaleString()}
                              </td>
                              <td className="py-2">{trade.quantity}</td>
                              <td className="py-2">
                                {formatCurrency(trade.price)}
                              </td>
                              <td className="py-2">
                                {formatCurrency(trade.commission)}
                              </td>
                              <td className="py-2">
                                {formatCurrency(trade.quantity * trade.price)}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
