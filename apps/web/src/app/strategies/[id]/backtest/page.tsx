'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { ArrowLeft, Play, Calendar } from 'lucide-react'
import Link from 'next/link'

interface Strategy {
  id: string
  name: string
  description?: string
  author: string
  is_public: boolean
  created_at: string
  updated_at: string
}

interface StrategyVersion {
  id: string
  package_id: string
  version: string
  code: string
  description?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export default function BacktestPage() {
  const params = useParams()
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [strategy, setStrategy] = useState<Strategy | null>(null)
  const [versions, setVersions] = useState<StrategyVersion[]>([])
  const [formData, setFormData] = useState({
    name: '',
    symbols: '',
    start_date: '',
    end_date: '',
    parameters: '',
  })

  const strategyId = params.id as string

  useEffect(() => {
    if (strategyId) {
      fetchStrategy()
      fetchVersions()
    }
  }, [strategyId])

  const fetchStrategy = async () => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}`)
      if (!response.ok) {
        throw new Error('Failed to fetch strategy')
      }
      const data = await response.json()
      setStrategy(data.data)
      setFormData(prev => ({
        ...prev,
        name: `${data.data.name} Backtest`,
      }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    }
  }

  const fetchVersions = async () => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}/versions`)
      if (!response.ok) {
        throw new Error('Failed to fetch versions')
      }
      const data = await response.json()
      setVersions(data.data || [])
    } catch (err) {
      console.error('Failed to fetch versions:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setRunning(true)
    setError(null)

    try {
      // Parse symbols
      const symbols = formData.symbols.split(',').map(s => s.trim()).filter(s => s)

      // Parse parameters
      let parameters = null
      if (formData.parameters.trim()) {
        try {
          parameters = JSON.parse(formData.parameters)
        } catch (err) {
          throw new Error('Invalid JSON in parameters')
        }
      }

      const backtestData = {
        name: formData.name,
        strategy_id: strategyId,
        symbols,
        start_date: formData.start_date,
        end_date: formData.end_date,
        parameters,
      }

      const response = await fetch('/api/v1/backtests', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(backtestData),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to create backtest')
      }

      const data = await response.json()
      router.push(`/backtests/${data.data.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred')
    } finally {
      setRunning(false)
    }
  }

  const handleInputChange = (field: string, value: string) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }))
  }

  if (loading) {
    return (
      <div className="container mx-auto p-6 max-w-2xl">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading strategy...</div>
        </div>
      </div>
    )
  }

  if (error || !strategy) {
    return (
      <div className="container mx-auto p-6 max-w-2xl">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">Error: {error || 'Strategy not found'}</div>
        </div>
      </div>
    )
  }

  const activeVersion = versions.find(v => v.is_active)

  if (!activeVersion) {
    return (
      <div className="container mx-auto p-6 max-w-2xl">
        <div className="mb-6">
          <Link href={`/strategies/${strategyId}`} className="inline-flex items-center text-muted-foreground hover:text-foreground">
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back to Strategy
          </Link>
        </div>
        <Card>
          <CardContent className="flex flex-col items-center justify-center h-64">
            <div className="text-center">
              <h3 className="text-lg font-semibold mb-2">No Active Version</h3>
              <p className="text-muted-foreground mb-4">
                You need an active version to run a backtest.
              </p>
              <Link href={`/strategies/${strategyId}/versions/new`}>
                <Button>
                  Create Version
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="container mx-auto p-6 max-w-2xl">
      <div className="mb-6">
        <Link href={`/strategies/${strategyId}`} className="inline-flex items-center text-muted-foreground hover:text-foreground">
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Strategy
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Run Backtest</CardTitle>
          <CardDescription>
            Test your strategy "{strategy.name}" with historical data.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="name">Backtest Name *</Label>
              <Input
                id="name"
                value={formData.name}
                onChange={(e) => handleInputChange('name', e.target.value)}
                placeholder="e.g., EMA Cross Strategy Backtest"
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="symbols">Symbols *</Label>
              <Input
                id="symbols"
                value={formData.symbols}
                onChange={(e) => handleInputChange('symbols', e.target.value)}
                placeholder="e.g., AAPL, GOOGL, MSFT"
                required
              />
              <p className="text-sm text-muted-foreground">
                Enter comma-separated symbols to test
              </p>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="start_date">Start Date *</Label>
                <Input
                  id="start_date"
                  type="date"
                  value={formData.start_date}
                  onChange={(e) => handleInputChange('start_date', e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="end_date">End Date *</Label>
                <Input
                  id="end_date"
                  type="date"
                  value={formData.end_date}
                  onChange={(e) => handleInputChange('end_date', e.target.value)}
                  required
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="parameters">Parameters (JSON)</Label>
              <Textarea
                id="parameters"
                value={formData.parameters}
                onChange={(e) => handleInputChange('parameters', e.target.value)}
                placeholder='{"fast_period": 12, "slow_period": 26}'
                rows={4}
              />
              <p className="text-sm text-muted-foreground">
                Optional JSON parameters for your strategy
              </p>
            </div>

            <div className="p-4 bg-muted rounded-lg">
              <h4 className="font-medium mb-2">Strategy Information</h4>
              <div className="space-y-1 text-sm">
                <p><span className="font-medium">Strategy:</span> {strategy.name}</p>
                <p><span className="font-medium">Version:</span> {activeVersion.version}</p>
                <p><span className="font-medium">Active Version:</span> {activeVersion.is_active ? 'Yes' : 'No'}</p>
              </div>
            </div>

            {error && (
              <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                <p className="text-red-600 text-sm">{error}</p>
              </div>
            )}

            <div className="flex gap-3">
              <Button type="submit" disabled={running}>
                {running ? (
                  'Running Backtest...'
                ) : (
                  <>
                    <Play className="w-4 h-4 mr-2" />
                    Run Backtest
                  </>
                )}
              </Button>
              <Link href={`/strategies/${strategyId}`}>
                <Button type="button" variant="outline">
                  Cancel
                </Button>
              </Link>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}