'use client';

import { useEffect, useState } from 'react';
import type { Quote } from '@/lib/chart';

const GMO_PUBLIC_WS = 'wss://forex-api.coin.z.com/ws/public/v1';
/** Quotes arrive many times a second; the page does not need to redraw that often. */
const PUBLISH_MS = 250;
const MAX_RETRY_MS = 30000;

export type TickerState = 'connecting' | 'live' | 'closed';

/**
 * Streams a symbol's quote straight from GMO's public WebSocket (no key).
 * `closed` means the market is closed or the connection is down; it reconnects
 * by itself.
 */
export function useGmoTicker(symbol: string): {
  quote: Quote | null;
  state: TickerState;
} {
  const [quote, setQuote] = useState<Quote | null>(null);
  const [state, setState] = useState<TickerState>('connecting');

  useEffect(() => {
    let socket: WebSocket | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let delay = 2000; // GMO allows one subscribe per second per IP
    let latest: Quote | null = null;
    let stopped = false;

    setQuote(null);
    setState('connecting');

    const publish = setInterval(() => {
      if (latest) {
        setQuote(latest);
        latest = null;
      }
    }, PUBLISH_MS);

    const connect = () => {
      socket = new WebSocket(GMO_PUBLIC_WS);
      socket.onopen = () => {
        socket?.send(
          JSON.stringify({ command: 'subscribe', channel: 'ticker', symbol })
        );
      };
      socket.onmessage = event => {
        let msg: Record<string, string>;
        try {
          msg = JSON.parse(event.data);
        } catch {
          return;
        }
        const bid = Number(msg.bid);
        const ask = Number(msg.ask);
        const time = Date.parse(msg.timestamp ?? '') / 1000;
        if (msg.symbol !== symbol || !bid || !ask || !time) {
          return;
        }
        if (msg.status !== 'OPEN') {
          setState('closed');
          return;
        }
        delay = 2000;
        latest = { bid, ask, time };
        setState('live');
      };
      socket.onclose = () => {
        if (stopped) {
          return;
        }
        setState('closed');
        retry = setTimeout(connect, delay);
        delay = Math.min(delay * 2, MAX_RETRY_MS);
      };
    };
    connect();

    return () => {
      stopped = true;
      clearInterval(publish);
      clearTimeout(retry);
      socket?.close();
    };
  }, [symbol]);

  return { quote, state };
}
