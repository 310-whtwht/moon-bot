'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import {
  BarChart3,
  Bell,
  Globe,
  Home,
  ListOrdered,
  Shield,
  TrendingUp,
} from 'lucide-react';

// Only link to pages that exist under src/app.
const navigation = [
  { name: 'ダッシュボード', href: '/dashboard', icon: Home },
  { name: '戦略管理', href: '/strategies', icon: TrendingUp },
  { name: 'バックテスト', href: '/backtests', icon: BarChart3 },
  { name: '注文', href: '/orders', icon: ListOrdered },
  { name: 'ユニバース', href: '/universe', icon: Globe },
  { name: '監査・ログ', href: '/audit', icon: Shield },
  { name: '通知', href: '/notifications', icon: Bell },
];

export default function Navigation() {
  const pathname = usePathname();

  return (
    <nav className="flex space-x-4 lg:space-x-6">
      {navigation.map(item => {
        const isActive =
          pathname === item.href || pathname.startsWith(`${item.href}/`);
        return (
          <Link
            key={item.name}
            href={item.href}
            className={cn(
              'flex items-center space-x-2 text-sm font-medium transition-colors hover:text-primary',
              isActive ? 'text-black dark:text-white' : 'text-muted-foreground'
            )}
          >
            <item.icon className="h-4 w-4" />
            <span className="hidden md:inline-block">{item.name}</span>
          </Link>
        );
      })}
    </nav>
  );
}
