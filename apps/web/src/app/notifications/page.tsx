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
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Bell,
  Mail,
  MessageSquare,
  Settings,
  Trash2,
  CheckCircle,
  AlertTriangle,
  Info,
  Zap,
  Shield,
  DollarSign,
  BarChart3,
} from 'lucide-react';

interface Notification {
  id: string;
  type: 'info' | 'warning' | 'error' | 'success';
  title: string;
  message: string;
  category: 'system' | 'trading' | 'security' | 'performance';
  timestamp: string;
  read: boolean;
  priority: 'low' | 'medium' | 'high';
}

interface NotificationSettings {
  email: boolean;
  slack: boolean;
  webhook: boolean;
  categories: {
    system: boolean;
    trading: boolean;
    security: boolean;
    performance: boolean;
  };
  priorities: {
    low: boolean;
    medium: boolean;
    high: boolean;
  };
}

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [settings, setSettings] = useState<NotificationSettings>({
    email: true,
    slack: false,
    webhook: false,
    categories: {
      system: true,
      trading: true,
      security: true,
      performance: true,
    },
    priorities: {
      low: true,
      medium: true,
      high: true,
    },
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchNotifications();
  }, []);

  const fetchNotifications = async () => {
    try {
      setLoading(true);

      // Mock notifications data
      const mockNotifications: Notification[] = [
        {
          id: '1',
          type: 'success',
          title: 'Strategy Execution Successful',
          message:
            'EMA Cross strategy executed 5 trades successfully today with 80% win rate.',
          category: 'trading',
          timestamp: new Date(Date.now() - 300000).toISOString(),
          read: false,
          priority: 'medium',
        },
        {
          id: '2',
          type: 'warning',
          title: 'High Volatility Detected',
          message:
            'Unusual volatility detected in AAPL. Consider reviewing position sizing.',
          category: 'performance',
          timestamp: new Date(Date.now() - 600000).toISOString(),
          read: false,
          priority: 'high',
        },
        {
          id: '3',
          type: 'error',
          title: 'Connection Lost',
          message: 'Connection to moomoo API lost. Attempting to reconnect...',
          category: 'system',
          timestamp: new Date(Date.now() - 900000).toISOString(),
          read: true,
          priority: 'high',
        },
        {
          id: '4',
          type: 'info',
          title: 'Daily Summary',
          message:
            'Daily trading summary: 12 trades, $1,250 P&L, 75% win rate.',
          category: 'performance',
          timestamp: new Date(Date.now() - 3600000).toISOString(),
          read: true,
          priority: 'low',
        },
        {
          id: '5',
          type: 'warning',
          title: 'Risk Limit Approaching',
          message:
            'Daily drawdown approaching 2% limit. Consider reducing position sizes.',
          category: 'security',
          timestamp: new Date(Date.now() - 7200000).toISOString(),
          read: false,
          priority: 'high',
        },
      ];

      setNotifications(mockNotifications);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  const markAsRead = async (id: string) => {
    setNotifications(prev =>
      prev.map(notification =>
        notification.id === id ? { ...notification, read: true } : notification
      )
    );
  };

  const deleteNotification = async (id: string) => {
    setNotifications(prev =>
      prev.filter(notification => notification.id !== id)
    );
  };

  const markAllAsRead = async () => {
    setNotifications(prev =>
      prev.map(notification => ({ ...notification, read: true }))
    );
  };

  const getNotificationIcon = (type: string) => {
    switch (type) {
      case 'success':
        return <CheckCircle className="w-4 h-4 text-green-600" />;
      case 'warning':
        return <AlertTriangle className="w-4 h-4 text-yellow-600" />;
      case 'error':
        return <AlertTriangle className="w-4 h-4 text-red-600" />;
      case 'info':
        return <Info className="w-4 h-4 text-blue-600" />;
      default:
        return <Bell className="w-4 h-4" />;
    }
  };

  const getCategoryIcon = (category: string) => {
    switch (category) {
      case 'system':
        return <Zap className="w-4 h-4" />;
      case 'trading':
        return <DollarSign className="w-4 h-4" />;
      case 'security':
        return <Shield className="w-4 h-4" />;
      case 'performance':
        return <BarChart3 className="w-4 h-4" />;
      default:
        return <Bell className="w-4 h-4" />;
    }
  };

  const getPriorityColor = (priority: string) => {
    switch (priority) {
      case 'high':
        return 'bg-red-100 text-red-800';
      case 'medium':
        return 'bg-yellow-100 text-yellow-800';
      case 'low':
        return 'bg-green-100 text-green-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const unreadCount = notifications.filter(n => !n.read).length;

  if (loading) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-lg">Loading notifications...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container mx-auto p-6">
        <div className="flex items-center justify-center h-64">
          <div className="text-red-500">Error: {error}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-3xl font-bold">Notifications</h1>
          <p className="text-muted-foreground">
            Manage your notification preferences and view alerts
          </p>
        </div>
        <div className="flex gap-2">
          {unreadCount > 0 && (
            <Button variant="outline" onClick={markAllAsRead}>
              Mark All as Read
            </Button>
          )}
        </div>
      </div>

      <Tabs defaultValue="notifications" className="w-full">
        <TabsList>
          <TabsTrigger value="notifications">
            Notifications
            {unreadCount > 0 && (
              <Badge variant="destructive" className="ml-2">
                {unreadCount}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        <TabsContent value="notifications" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Recent Notifications</CardTitle>
              <CardDescription>
                {unreadCount > 0
                  ? `${unreadCount} unread notifications`
                  : 'All notifications read'}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {notifications.length === 0 ? (
                <div className="text-center py-8">
                  <Bell className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                  <h3 className="text-lg font-semibold mb-2">
                    No notifications
                  </h3>
                  <p className="text-muted-foreground">
                    You&apos;re all caught up!
                  </p>
                </div>
              ) : (
                <div className="space-y-4">
                  {notifications.map(notification => (
                    <div
                      key={notification.id}
                      className={`p-4 border rounded-lg ${!notification.read ? 'bg-blue-50 border-blue-200' : ''}`}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex items-start gap-3 flex-1">
                          {getNotificationIcon(notification.type)}
                          <div className="flex-1">
                            <div className="flex items-center gap-2 mb-1">
                              <h4 className="font-medium">
                                {notification.title}
                              </h4>
                              <Badge
                                className={getPriorityColor(
                                  notification.priority
                                )}
                              >
                                {notification.priority}
                              </Badge>
                              <Badge
                                variant="outline"
                                className="flex items-center gap-1"
                              >
                                {getCategoryIcon(notification.category)}
                                {notification.category}
                              </Badge>
                              {!notification.read && (
                                <Badge
                                  variant="default"
                                  className="bg-blue-600"
                                >
                                  New
                                </Badge>
                              )}
                            </div>
                            <p className="text-sm text-muted-foreground mb-2">
                              {notification.message}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {new Date(
                                notification.timestamp
                              ).toLocaleString()}
                            </p>
                          </div>
                        </div>
                        <div className="flex gap-2">
                          {!notification.read && (
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => markAsRead(notification.id)}
                            >
                              Mark Read
                            </Button>
                          )}
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => deleteNotification(notification.id)}
                            className="text-red-600 hover:text-red-700"
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="settings" className="mt-6">
          <div className="grid gap-6 md:grid-cols-2">
            {/* Notification Channels */}
            <Card>
              <CardHeader>
                <CardTitle>Notification Channels</CardTitle>
                <CardDescription>
                  Choose how you want to receive notifications
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Mail className="w-4 h-4" />
                    <Label htmlFor="email">Email Notifications</Label>
                  </div>
                  <Switch
                    id="email"
                    checked={settings.email}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({ ...prev, email: checked }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <MessageSquare className="w-4 h-4" />
                    <Label htmlFor="slack">Slack Notifications</Label>
                  </div>
                  <Switch
                    id="slack"
                    checked={settings.slack}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({ ...prev, slack: checked }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Settings className="w-4 h-4" />
                    <Label htmlFor="webhook">Webhook Notifications</Label>
                  </div>
                  <Switch
                    id="webhook"
                    checked={settings.webhook}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({ ...prev, webhook: checked }))
                    }
                  />
                </div>
              </CardContent>
            </Card>

            {/* Notification Categories */}
            <Card>
              <CardHeader>
                <CardTitle>Notification Categories</CardTitle>
                <CardDescription>
                  Select which types of notifications to receive
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Zap className="w-4 h-4" />
                    <Label htmlFor="system">System Notifications</Label>
                  </div>
                  <Switch
                    id="system"
                    checked={settings.categories.system}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        categories: { ...prev.categories, system: checked },
                      }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <DollarSign className="w-4 h-4" />
                    <Label htmlFor="trading">Trading Notifications</Label>
                  </div>
                  <Switch
                    id="trading"
                    checked={settings.categories.trading}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        categories: { ...prev.categories, trading: checked },
                      }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Shield className="w-4 h-4" />
                    <Label htmlFor="security">Security Notifications</Label>
                  </div>
                  <Switch
                    id="security"
                    checked={settings.categories.security}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        categories: { ...prev.categories, security: checked },
                      }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <BarChart3 className="w-4 h-4" />
                    <Label htmlFor="performance">
                      Performance Notifications
                    </Label>
                  </div>
                  <Switch
                    id="performance"
                    checked={settings.categories.performance}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        categories: {
                          ...prev.categories,
                          performance: checked,
                        },
                      }))
                    }
                  />
                </div>
              </CardContent>
            </Card>

            {/* Priority Levels */}
            <Card>
              <CardHeader>
                <CardTitle>Priority Levels</CardTitle>
                <CardDescription>
                  Choose which priority levels to receive
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge className="bg-red-100 text-red-800">High</Badge>
                    <Label htmlFor="high">High Priority</Label>
                  </div>
                  <Switch
                    id="high"
                    checked={settings.priorities.high}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        priorities: { ...prev.priorities, high: checked },
                      }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge className="bg-yellow-100 text-yellow-800">
                      Medium
                    </Badge>
                    <Label htmlFor="medium">Medium Priority</Label>
                  </div>
                  <Switch
                    id="medium"
                    checked={settings.priorities.medium}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        priorities: { ...prev.priorities, medium: checked },
                      }))
                    }
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge className="bg-green-100 text-green-800">Low</Badge>
                    <Label htmlFor="low">Low Priority</Label>
                  </div>
                  <Switch
                    id="low"
                    checked={settings.priorities.low}
                    onCheckedChange={(checked: boolean) =>
                      setSettings(prev => ({
                        ...prev,
                        priorities: { ...prev.priorities, low: checked },
                      }))
                    }
                  />
                </div>
              </CardContent>
            </Card>

            {/* Webhook Configuration */}
            {settings.webhook && (
              <Card>
                <CardHeader>
                  <CardTitle>Webhook Configuration</CardTitle>
                  <CardDescription>
                    Configure webhook endpoint for notifications
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <Label htmlFor="webhook-url">Webhook URL</Label>
                    <Input
                      id="webhook-url"
                      placeholder="https://your-webhook-endpoint.com/notifications"
                    />
                  </div>
                  <div>
                    <Label htmlFor="webhook-secret">
                      Secret Key (Optional)
                    </Label>
                    <Input
                      id="webhook-secret"
                      type="password"
                      placeholder="Enter secret key for webhook authentication"
                    />
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
