'use client';

import { useEffect, useState } from 'react';
import { Moon, Sun } from 'lucide-react';
import { Button } from '@/components/ui/button';

export const THEME_KEY = 'theme';

/**
 * Runs before the first paint (see layout.tsx) so the page never flashes in
 * the wrong theme: the saved choice wins, otherwise the OS setting.
 */
export const THEME_INIT_SCRIPT = `(function(){try{var t=localStorage.getItem('${THEME_KEY}');var d=t?t==='dark':window.matchMedia('(prefers-color-scheme: dark)').matches;document.documentElement.classList.toggle('dark',d);}catch(e){}})();`;

/** Switches between the light and dark theme and remembers the choice. */
export function ThemeToggle() {
  // Unknown until mounted: the server cannot know the theme.
  const [dark, setDark] = useState<boolean | null>(null);

  useEffect(() => {
    setDark(document.documentElement.classList.contains('dark'));
  }, []);

  const toggle = () => {
    const next = !document.documentElement.classList.contains('dark');
    document.documentElement.classList.toggle('dark', next);
    try {
      localStorage.setItem(THEME_KEY, next ? 'dark' : 'light');
    } catch {
      // Private browsing: the choice just does not persist.
    }
    setDark(next);
  };

  const label = dark ? 'ライトモードに切り替え' : 'ダークモードに切り替え';
  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={toggle}
      title={label}
      aria-label={label}
    >
      {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  );
}
