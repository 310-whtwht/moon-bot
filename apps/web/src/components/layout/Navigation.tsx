'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import { 
  BarChart3, 
  Settings, 
  FileText, 
  Shield, 
  Home,
  TrendingUp
} from 'lucide-react'

const navigation = [
  { name: 'ダッシュボード', href: '/', icon: Home },
  { name: '戦略管理', href: '/strategies', icon: TrendingUp },
  { name: '監査・ログ', href: '/audit', icon: Shield },
  { name: '設定', href: '/settings', icon: Settings },
  { name: 'ドキュメント', href: '/docs', icon: FileText },
]

export default function Navigation() {
  const pathname = usePathname()

  return (
    <nav className="flex space-x-4 lg:space-x-6">
      {navigation.map((item) => {
        const isActive = pathname === item.href
        return (
          <Link
            key={item.name}
            href={item.href}
            className={cn(
              'flex items-center space-x-2 text-sm font-medium transition-colors hover:text-primary',
              isActive
                ? 'text-black dark:text-white'
                : 'text-muted-foreground'
            )}
          >
            <item.icon className="h-4 w-4" />
            <span className="hidden md:inline-block">{item.name}</span>
          </Link>
        )
      })}
    </nav>
  )
}