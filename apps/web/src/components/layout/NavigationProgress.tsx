'use client';

import { useEffect, useState } from 'react';
import { usePathname } from 'next/navigation';

/** Give up after this long, so a failed navigation does not leave the bar on. */
const TIMEOUT_MS = 20000;

/**
 * A bar under the header that runs from the moment a link is clicked until
 * the new page is on screen, so a slow page change never looks frozen.
 */
export default function NavigationProgress() {
  const pathname = usePathname();
  const [pending, setPending] = useState(false);

  useEffect(() => {
    setPending(false);
  }, [pathname]);

  useEffect(() => {
    const onClick = (event: MouseEvent) => {
      // next/link always calls preventDefault, so that cannot be used to skip.
      if (
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      ) {
        return;
      }
      const link = (event.target as Element | null)?.closest?.('a[href]');
      if (!(link instanceof HTMLAnchorElement) || link.target === '_blank') {
        return;
      }
      const url = new URL(link.href, window.location.href);
      if (
        url.origin === window.location.origin &&
        url.pathname !== window.location.pathname
      ) {
        setPending(true);
      }
    };
    document.addEventListener('click', onClick);
    return () => document.removeEventListener('click', onClick);
  }, []);

  useEffect(() => {
    if (!pending) {
      return;
    }
    const timer = setTimeout(() => setPending(false), TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [pending]);

  if (!pending) {
    return null;
  }
  return (
    <div
      role="progressbar"
      aria-label="画面を読み込み中"
      className="fixed inset-x-0 top-14 z-50 h-1 overflow-hidden bg-primary/15"
    >
      <div className="h-full w-1/3 animate-nav-progress rounded-full bg-primary" />
    </div>
  );
}
