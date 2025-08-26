'use client'

import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Plus, Search, Upload, Download, Globe, Building2, Coins, TrendingUp, Eye, Edit, Trash2 } from 'lucide-react'

interface UniverseSymbol {
  id: string
  symbol: string
  name?: string
  exchange: string
  asset_type: string
  is_active: boolean
  data_source: string
  last_updated?: string
  created_at: string
  updated_at: string
}

export default function UniversePage() {
  const [symbols, setSymbols] = useState<UniverseSymbol[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [exchangeFilter, setExchangeFilter] = useState('all')
  const [assetTypeFilter, setAssetTypeFilter] = useState('all')
  const [activeFilter, setActiveFilter] = useState('all')
  const [showAddForm, setShowAddForm] = useState(false)
  const [showBulkUpload, setShowBulkUpload] = useState(false)
  const [newSymbol, setNewSymbol] = useState({
    symbol: '',
    name: '',
    exchange: 'NASDAQ',
    asset_type: 'stock',
    is_active: true,
    data_source: 'moomoo'
  })
  const [bulkSymbols, setBulkSymbols] = useState('')

  useEffect(() => {
    fetchUniverse()
  }, [])

  const fetchUniverse = async () => {
    try {
      setLoading(true)
      const response = await fetch('/api/v1/universe')
      if (!response.ok) {
        throw new Error('Failed to fetch universe')
      }
      const data = await response.json()
      setSymbols(data.data || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setLoading(false)
    }
  }

  const handleAddSymbol = async () => {
    try {
      const response = await fetch('/api/v1/universe', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newSymbol),
      })
      if (!response.ok) {
        throw new Error('Failed to add symbol')
      }
      await fetchUniverse()
      setNewSymbol({
        symbol: '',
        name: '',
        exchange: 'NASDAQ',
        asset_type: 'stock',
        is_active: true,
        data_source: 'moomoo'
      })
      setShowAddForm(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add symbol')
    }
  }

  const handleBulkUpload = async () => {
    try {
      const symbolsList = bulkSymbols.split('\n').filter(line => line.trim()).map(line => {
        const [symbol, name, exchange = 'NASDAQ', assetType = 'stock'] = line.split(',').map(s => s.trim())
        return {
          symbol,
          name: name || symbol,
          exchange,
          asset_type: assetType,
          is_active: true,
          data_source: 'moomoo'
        }
      })

      const response = await fetch('/api/v1/universe/bulk', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ symbols: symbolsList }),
      })
      if (!response.ok) {
        throw new Error('Failed to bulk upload symbols')
      }
      await fetchUniverse()
      setBulkSymbols('')
      setShowBulkUpload(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to bulk upload symbols')
    }
  }

  const handleDeleteSymbol = async (symbol: string) => {
    if (!confirm(`Are you sure you want to remove ${symbol} from the universe?`)) {
      return
    }

    try {
      const response = await fetch(`/api/v1/universe/${symbol}`, {
        method: 'DELETE',
      })
      if (!response.ok) {
        throw new Error('Failed to delete symbol')
      }
      await fetchUniverse()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete symbol')
    }
  }

  const handleToggleActive = async (symbol: UniverseSymbol) => {
    try {
      const response = await fetch(`/api/v1/universe/${symbol.symbol}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          ...symbol,
          is_active: !symbol.is_active
        }),
      })
      if (!response.ok) {
        throw new Error('Failed to update symbol')
      }
      await fetchUniverse()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update symbol')
    }
  }

  const getAssetTypeIcon = (assetType: string) => {
    switch (assetType) {
      case 'stock':
        return <Building2 className="w-4 h-4" />
      case 'etf':
        return <TrendingUp className="w-4 h-4" />
      case 'crypto':
        return <Coins className="w-4 h-4" />
      default:
        return <Globe className="w-4 h-4" />
    }
  }

  const getAssetTypeColor = (assetType: string) => {
    switch (assetType) {
      case 'stock':
        return 'bg-blue-100 text-blue-800'
      case 'etf':
        return 'bg-green-100 text-green-800'
      case 'crypto':
        return 'bg-yellow-100 text-yellow-800'
      case 'option':
        return 'bg-purple-100 text-purple-800'
      case 'future':
        return 'bg-orange-100 text-orange-800'
      default:
        return 'bg-gray-100 text-gray-800'
    }
  }

  const filteredSymbols = symbols.filter(symbol => {
    const matchesSearch = symbol.symbol.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         (symbol.name && symbol.name.toLowerCase().includes(searchTerm.toLowerCase()))
    const matchesExchange = exchangeFilter === 'all' || symbol.exchange === exchangeFilter
    const matchesAssetType = assetTypeFilter === 'all' || symbol.asset_type === assetTypeFilter
    const matchesActive = activeFilter === 'all' || 
                         (activeFilter === 'active' && symbol.is_active) ||
                         (activeFilter === 'inactive' && !symbol.is_active)
    return matchesSearch && matchesExchange && matchesAssetType && matchesActive
  })

  const exchanges = [...new Set(symbols.map(s => s.exchange))]
  const assetTypes = [...new Set(symbols.map(s => s.asset_type))]

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading universe...</div>
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
          <h1 className="text-3xl font-bold">Universe Management</h1>
          <p className="text-muted-foreground">Manage tradable symbols and assets</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setShowBulkUpload(!showBulkUpload)}>
            <Upload className="w-4 h-4 mr-2" />
            Bulk Upload
          </Button>
          <Button onClick={() => setShowAddForm(!showAddForm)}>
            <Plus className="w-4 h-4 mr-2" />
            Add Symbol
          </Button>
        </div>
      </div>

      {/* Add Symbol Form */}
      {showAddForm && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Add New Symbol</CardTitle>
            <CardDescription>Add a single symbol to the universe</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 md:grid-cols-2">
              <div>
                <Label htmlFor="symbol">Symbol</Label>
                <Input
                  id="symbol"
                  value={newSymbol.symbol}
                  onChange={(e) => setNewSymbol({ ...newSymbol, symbol: e.target.value })}
                  placeholder="AAPL"
                />
              </div>
              <div>
                <Label htmlFor="name">Name (Optional)</Label>
                <Input
                  id="name"
                  value={newSymbol.name}
                  onChange={(e) => setNewSymbol({ ...newSymbol, name: e.target.value })}
                  placeholder="Apple Inc."
                />
              </div>
              <div>
                <Label htmlFor="exchange">Exchange</Label>
                <select
                  id="exchange"
                  value={newSymbol.exchange}
                  onChange={(e) => setNewSymbol({ ...newSymbol, exchange: e.target.value })}
                  className="w-full px-3 py-2 border border-input rounded-md bg-background"
                >
                  <option value="NASDAQ">NASDAQ</option>
                  <option value="NYSE">NYSE</option>
                  <option value="AMEX">AMEX</option>
                  <option value="BINANCE">BINANCE</option>
                  <option value="COINBASE">COINBASE</option>
                </select>
              </div>
              <div>
                <Label htmlFor="asset_type">Asset Type</Label>
                <select
                  id="asset_type"
                  value={newSymbol.asset_type}
                  onChange={(e) => setNewSymbol({ ...newSymbol, asset_type: e.target.value })}
                  className="w-full px-3 py-2 border border-input rounded-md bg-background"
                >
                  <option value="stock">Stock</option>
                  <option value="etf">ETF</option>
                  <option value="crypto">Crypto</option>
                  <option value="option">Option</option>
                  <option value="future">Future</option>
                </select>
              </div>
              <div className="flex items-center space-x-2">
                <Switch
                  id="is_active"
                  checked={newSymbol.is_active}
                  onCheckedChange={(checked) => setNewSymbol({ ...newSymbol, is_active: checked })}
                />
                <Label htmlFor="is_active">Active</Label>
              </div>
            </div>
            <div className="flex gap-2 mt-4">
              <Button onClick={handleAddSymbol}>Add Symbol</Button>
              <Button variant="outline" onClick={() => setShowAddForm(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Bulk Upload Form */}
      {showBulkUpload && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Bulk Upload Symbols</CardTitle>
            <CardDescription>
              Upload multiple symbols at once. Format: symbol,name,exchange,asset_type (one per line)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Textarea
              value={bulkSymbols}
              onChange={(e) => setBulkSymbols(e.target.value)}
              placeholder="AAPL,Apple Inc.,NASDAQ,stock&#10;TSLA,Tesla Inc.,NASDAQ,stock&#10;BTC-USD,Bitcoin,BINANCE,crypto"
              rows={6}
            />
            <div className="flex gap-2 mt-4">
              <Button onClick={handleBulkUpload}>Upload Symbols</Button>
              <Button variant="outline" onClick={() => setShowBulkUpload(false)}>Cancel</Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Filters */}
      <div className="mb-6 flex gap-4">
        <div className="flex-1">
          <Input
            placeholder="Search symbols..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="max-w-sm"
          />
        </div>
        <select
          value={exchangeFilter}
          onChange={(e) => setExchangeFilter(e.target.value)}
          className="px-3 py-2 border border-input rounded-md bg-background"
        >
          <option value="all">All Exchanges</option>
          {exchanges.map(exchange => (
            <option key={exchange} value={exchange}>{exchange}</option>
          ))}
        </select>
        <select
          value={assetTypeFilter}
          onChange={(e) => setAssetTypeFilter(e.target.value)}
          className="px-3 py-2 border border-input rounded-md bg-background"
        >
          <option value="all">All Types</option>
          {assetTypes.map(type => (
            <option key={type} value={type}>{type}</option>
          ))}
        </select>
        <select
          value={activeFilter}
          onChange={(e) => setActiveFilter(e.target.value)}
          className="px-3 py-2 border border-input rounded-md bg-background"
        >
          <option value="all">All Status</option>
          <option value="active">Active Only</option>
          <option value="inactive">Inactive Only</option>
        </select>
      </div>

      {/* Statistics */}
      <div className="grid gap-4 mb-6 md:grid-cols-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Total Symbols</p>
                <p className="text-2xl font-bold">{symbols.length}</p>
              </div>
              <Globe className="w-8 h-8 text-muted-foreground" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Active Symbols</p>
                <p className="text-2xl font-bold">{symbols.filter(s => s.is_active).length}</p>
              </div>
              <Eye className="w-8 h-8 text-green-600" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Exchanges</p>
                <p className="text-2xl font-bold">{exchanges.length}</p>
              </div>
              <Building2 className="w-8 h-8 text-blue-600" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-muted-foreground">Asset Types</p>
                <p className="text-2xl font-bold">{assetTypes.length}</p>
              </div>
              <Coins className="w-8 h-8 text-yellow-600" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Symbols List */}
      {filteredSymbols.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center h-64">
            <Globe className="w-12 h-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No symbols found</h3>
            <p className="text-muted-foreground mb-4">
              {searchTerm || exchangeFilter !== 'all' || assetTypeFilter !== 'all' || activeFilter !== 'all'
                ? 'Try adjusting your search or filters'
                : 'Add your first symbol to get started'
              }
            </p>
            {!searchTerm && exchangeFilter === 'all' && assetTypeFilter === 'all' && activeFilter === 'all' && (
              <Button onClick={() => setShowAddForm(true)}>
                <Plus className="w-4 h-4 mr-2" />
                Add Symbol
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {filteredSymbols.map((symbol) => (
            <Card key={symbol.id} className="hover:shadow-lg transition-shadow">
              <CardHeader>
                <div className="flex justify-between items-start">
                  <div>
                    <CardTitle className="text-lg">{symbol.symbol}</CardTitle>
                    <CardDescription className="mt-1">
                      {symbol.name || 'No name provided'}
                    </CardDescription>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge className={getAssetTypeColor(symbol.asset_type)}>
                      {getAssetTypeIcon(symbol.asset_type)}
                      <span className="ml-1">{symbol.asset_type}</span>
                    </Badge>
                    <Badge variant={symbol.is_active ? 'default' : 'secondary'}>
                      {symbol.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  <div className="text-sm text-muted-foreground">
                    <span className="font-medium">Exchange:</span> {symbol.exchange}
                  </div>
                  <div className="text-sm text-muted-foreground">
                    <span className="font-medium">Data Source:</span> {symbol.data_source}
                  </div>
                  {symbol.last_updated && (
                    <div className="text-sm text-muted-foreground">
                      <span className="font-medium">Last Updated:</span>{' '}
                      {new Date(symbol.last_updated).toLocaleDateString()}
                    </div>
                  )}
                  <div className="flex gap-2 pt-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleToggleActive(symbol)}
                    >
                      {symbol.is_active ? 'Deactivate' : 'Activate'}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleDeleteSymbol(symbol.symbol)}
                      className="text-red-600 hover:text-red-700"
                    >
                      <Trash2 className="w-4 h-4 mr-1" />
                      Remove
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}