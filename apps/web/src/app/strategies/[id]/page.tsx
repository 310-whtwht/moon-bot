'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
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
import { ArrowLeft, Edit, Play, Plus, Trash2, Code } from 'lucide-react';
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

interface StrategyVersion {
  id: string;
  package_id: string;
  version: string;
  code: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function StrategyDetailPage() {
  const params = useParams();
  const router = useRouter();
  const [strategy, setStrategy] = useState<Strategy | null>(null);
  const [versions, setVersions] = useState<StrategyVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const strategyId = params.id as string;

  const fetchStrategy = useCallback(async () => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}`);
      if (!response.ok) {
        throw new Error('Failed to fetch strategy');
      }
      const data = await response.json();
      setStrategy(data.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    }
  }, [strategyId]);

  const fetchVersions = useCallback(async () => {
    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}/versions`);
      if (!response.ok) {
        throw new Error('Failed to fetch versions');
      }
      const data = await response.json();
      setVersions(data.data || []);
    } catch (err) {
      console.error('Failed to fetch versions:', err);
    } finally {
      setLoading(false);
    }
  }, [strategyId]);

  useEffect(() => {
    if (strategyId) {
      fetchStrategy();
      fetchVersions();
    }
  }, [strategyId, fetchStrategy, fetchVersions]);

  const handleDeleteStrategy = async () => {
    if (
      !window.confirm(
        'Are you sure you want to delete this strategy? This action cannot be undone.'
      )
    ) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/strategies/${strategyId}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        throw new Error('Failed to delete strategy');
      }
      router.push('/strategies');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to delete strategy'
      );
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading strategy...</div>
        </div>
      </div>
    );
  }

  if (error || !strategy) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">
            Error: {error || 'Strategy not found'}
          </div>
        </div>
      </div>
    );
  }

  const activeVersion = versions.find(v => v.is_active);

  return (
    <div className="container mx-auto p-6">
      <div className="mb-6">
        <Link
          href="/strategies"
          className="inline-flex items-center text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Strategies
        </Link>
      </div>

      <div className="flex justify-between items-start mb-6">
        <div>
          <h1 className="text-3xl font-bold">{strategy.name}</h1>
          <p className="text-muted-foreground mt-2">
            {strategy.description || 'No description'}
          </p>
        </div>
        <div className="flex gap-2">
          <Link href={`/strategies/${strategyId}/edit`}>
            <Button variant="outline">
              <Edit className="w-4 h-4 mr-2" />
              Edit
            </Button>
          </Link>
          <Link href={`/strategies/${strategyId}/backtest`}>
            <Button>
              <Play className="w-4 h-4 mr-2" />
              Backtest
            </Button>
          </Link>
          <Button
            variant="outline"
            onClick={handleDeleteStrategy}
            className="text-red-600 hover:text-red-700"
          >
            <Trash2 className="w-4 h-4 mr-2" />
            Delete
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Strategy Info */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle>Strategy Information</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Status
                </label>
                <div className="mt-1">
                  <Badge variant={strategy.is_public ? 'default' : 'secondary'}>
                    {strategy.is_public ? 'Public' : 'Private'}
                  </Badge>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Author
                </label>
                <p className="mt-1">{strategy.author}</p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Created
                </label>
                <p className="mt-1">
                  {new Date(strategy.created_at).toLocaleDateString()}
                </p>
              </div>
              <div>
                <label className="text-sm font-medium text-muted-foreground">
                  Last Updated
                </label>
                <p className="mt-1">
                  {new Date(strategy.updated_at).toLocaleDateString()}
                </p>
              </div>
              {activeVersion && (
                <div>
                  <label className="text-sm font-medium text-muted-foreground">
                    Active Version
                  </label>
                  <p className="mt-1">{activeVersion.version}</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Main Content */}
        <div className="lg:col-span-2">
          <Tabs defaultValue="versions" className="w-full">
            <TabsList>
              <TabsTrigger value="versions">Versions</TabsTrigger>
              <TabsTrigger value="code">Code</TabsTrigger>
              <TabsTrigger value="backtests">Backtests</TabsTrigger>
            </TabsList>

            <TabsContent value="versions" className="mt-6">
              <Card>
                <CardHeader>
                  <div className="flex justify-between items-center">
                    <CardTitle>Versions</CardTitle>
                    <Link href={`/strategies/${strategyId}/versions/new`}>
                      <Button size="sm">
                        <Plus className="w-4 h-4 mr-2" />
                        New Version
                      </Button>
                    </Link>
                  </div>
                </CardHeader>
                <CardContent>
                  {versions.length === 0 ? (
                    <div className="text-center py-8">
                      <Code className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                      <h3 className="text-lg font-semibold mb-2">
                        No versions yet
                      </h3>
                      <p className="text-muted-foreground mb-4">
                        Create your first version to start coding your strategy
                      </p>
                      <Link href={`/strategies/${strategyId}/versions/new`}>
                        <Button>
                          <Plus className="w-4 h-4 mr-2" />
                          Create Version
                        </Button>
                      </Link>
                    </div>
                  ) : (
                    <div className="space-y-4">
                      {versions.map(version => (
                        <div
                          key={version.id}
                          className="flex items-center justify-between p-4 border rounded-lg"
                        >
                          <div>
                            <div className="flex items-center gap-2">
                              <h4 className="font-medium">{version.version}</h4>
                              {version.is_active && (
                                <Badge variant="default">Active</Badge>
                              )}
                            </div>
                            <p className="text-sm text-muted-foreground mt-1">
                              {version.description || 'No description'}
                            </p>
                            <p className="text-xs text-muted-foreground mt-1">
                              Created{' '}
                              {new Date(
                                version.created_at
                              ).toLocaleDateString()}
                            </p>
                          </div>
                          <div className="flex gap-2">
                            <Link
                              href={`/strategies/${strategyId}/versions/${version.id}`}
                            >
                              <Button variant="outline" size="sm">
                                <Edit className="w-4 h-4 mr-1" />
                                Edit
                              </Button>
                            </Link>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="code" className="mt-6">
              <Card>
                <CardHeader>
                  <CardTitle>Strategy Code</CardTitle>
                  <CardDescription>
                    {activeVersion
                      ? `Active version: ${activeVersion.version}`
                      : 'No active version'}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {activeVersion ? (
                    <pre className="bg-muted p-4 rounded-lg overflow-x-auto text-sm">
                      <code>{activeVersion.code}</code>
                    </pre>
                  ) : (
                    <div className="text-center py-8">
                      <Code className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                      <p className="text-muted-foreground">
                        No active version to display
                      </p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="backtests" className="mt-6">
              <Card>
                <CardHeader>
                  <div className="flex justify-between items-center">
                    <CardTitle>Backtests</CardTitle>
                    <Link href={`/strategies/${strategyId}/backtest`}>
                      <Button size="sm">
                        <Play className="w-4 h-4 mr-2" />
                        Run Backtest
                      </Button>
                    </Link>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="text-center py-8">
                    <Play className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                    <h3 className="text-lg font-semibold mb-2">
                      No backtests yet
                    </h3>
                    <p className="text-muted-foreground mb-4">
                      Run your first backtest to see how your strategy performs
                    </p>
                    <Link href={`/strategies/${strategyId}/backtest`}>
                      <Button>
                        <Play className="w-4 h-4 mr-2" />
                        Run Backtest
                      </Button>
                    </Link>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
