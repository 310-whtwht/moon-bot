'use client';

import { useEffect, useRef } from 'react';
import {
  CandlestickSeries,
  ColorType,
  type IChartApi,
  type IPriceLine,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  LineSeries,
  LineStyle,
  type SeriesMarker,
  type Time,
  type UTCTimestamp,
  createChart,
  createSeriesMarkers,
} from 'lightweight-charts';
import type { ChartBar } from '@/lib/chart';

export interface ChartLine {
  label: string;
  color: string;
  values: (number | null)[];
}

export interface ChartMarker {
  /** Unix seconds (UTC); snapped to the bar it falls in. */
  time: number;
  kind: 'buy' | 'sell' | 'exit';
  text: string;
  /** A replayed signal rather than a real fill: drawn smaller, in its own colour. */
  hypothetical?: boolean;
}

export interface ChartLevel {
  price: number;
  label: string;
  color: string;
  dashed?: boolean;
}

interface Props {
  /** Changes when the history is replaced; otherwise only the last bar moves. */
  historyKey: string;
  bars: ChartBar[];
  barSeconds: number;
  lines: ChartLine[];
  markers: ChartMarker[];
  levels: ChartLevel[];
  height?: number;
}

// The library draws times as UTC: shift them so the axis reads in JST.
const JST_OFFSET = 9 * 3600;
const at = (unix: number) => (unix + JST_OFFSET) as UTCTimestamp;

const UP = '#16a34a';
const DOWN = '#dc2626';
/** Replayed signals, so they cannot be mistaken for real fills. */
export const SIGNAL_COLOR = '#7c3aed';
const SIGNAL = SIGNAL_COLOR;

const candle = (b: ChartBar) => ({
  time: at(b.time),
  open: b.open,
  high: b.high,
  low: b.low,
  close: b.close,
});

/** Candles with indicator lines, fill markers and price levels. */
export function LiveChart({
  historyKey,
  bars,
  barSeconds,
  lines,
  markers,
  levels,
  height = 480,
}: Props) {
  const container = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const candles = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const markerLayer = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const lineSeries = useRef<ISeriesApi<'Line'>[]>([]);
  const priceLines = useRef<IPriceLine[]>([]);
  const drawn = useRef<{ key: string; count: number }>({ key: '', count: 0 });

  useEffect(() => {
    if (!container.current) {
      return;
    }
    const api = createChart(container.current, {
      autoSize: true,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#6b7280',
      },
      grid: {
        vertLines: { color: 'rgba(148, 163, 184, 0.15)' },
        horzLines: { color: 'rgba(148, 163, 184, 0.15)' },
      },
      timeScale: { timeVisible: true, secondsVisible: false, rightOffset: 6 },
      localization: { locale: 'ja-JP' },
    });
    const series = api.addSeries(CandlestickSeries, {
      upColor: UP,
      downColor: DOWN,
      borderVisible: false,
      wickUpColor: UP,
      wickDownColor: DOWN,
      priceFormat: { type: 'price', precision: 3, minMove: 0.001 },
    });
    chart.current = api;
    candles.current = series;
    markerLayer.current = createSeriesMarkers(series, []);
    return () => {
      api.remove();
      chart.current = null;
      candles.current = null;
      markerLayer.current = null;
      lineSeries.current = [];
      priceLines.current = [];
      drawn.current = { key: '', count: 0 };
    };
  }, []);

  // Bars and indicator lines.
  useEffect(() => {
    const api = chart.current;
    const series = candles.current;
    if (!api || !series) {
      return;
    }
    while (lineSeries.current.length > lines.length) {
      api.removeSeries(lineSeries.current.pop()!);
    }
    lines.forEach((line, i) => {
      const options = {
        color: line.color,
        lineWidth: 2 as const,
        priceLineVisible: false,
        lastValueVisible: false,
        crosshairMarkerVisible: false,
      };
      if (lineSeries.current[i]) {
        lineSeries.current[i]!.applyOptions(options);
      } else {
        lineSeries.current[i] = api.addSeries(LineSeries, options);
      }
    });

    const linePoint = (line: ChartLine, i: number) => {
      const value = line.values[i];
      return value == null ? null : { time: at(bars[i]!.time), value };
    };

    const last = bars.length - 1;
    const sameHistory =
      drawn.current.key === historyKey &&
      last >= 0 &&
      (bars.length === drawn.current.count ||
        bars.length === drawn.current.count + 1);
    if (sameHistory) {
      series.update(candle(bars[last]!));
      lines.forEach((line, i) => {
        const point = linePoint(line, last);
        if (point) {
          lineSeries.current[i]!.update(point);
        }
      });
    } else {
      const firstDraw = drawn.current.count === 0;
      series.setData(bars.map(candle));
      lines.forEach((line, i) => {
        lineSeries.current[i]!.setData(
          bars.flatMap((_, j) => linePoint(line, j) ?? [])
        );
      });
      if (
        firstDraw ||
        drawn.current.key.split('|')[0] !== historyKey.split('|')[0]
      ) {
        // Show the latest ~120 bars; older ones are a scroll away.
        api.timeScale().setVisibleLogicalRange({
          from: Math.max(0, bars.length - 120),
          to: bars.length + 5,
        });
      }
    }
    drawn.current = { key: historyKey, count: bars.length };
  }, [historyKey, bars, lines]);

  // Fill markers.
  useEffect(() => {
    const first = bars[0]?.time;
    if (!markerLayer.current || first == null) {
      return;
    }
    const placed: SeriesMarker<Time>[] = markers
      .filter(m => m.time >= first)
      .map(m => {
        const time = at(Math.floor(m.time / barSeconds) * barSeconds);
        const size = m.hypothetical ? 1 : 1.4;
        if (m.kind === 'buy') {
          return {
            time,
            position: 'belowBar' as const,
            shape: 'arrowUp' as const,
            color: m.hypothetical ? SIGNAL : UP,
            text: m.text,
            size,
          };
        }
        if (m.kind === 'sell') {
          return {
            time,
            position: 'aboveBar' as const,
            shape: 'arrowDown' as const,
            color: m.hypothetical ? SIGNAL : DOWN,
            text: m.text,
            size,
          };
        }
        return {
          time,
          position: 'aboveBar' as const,
          shape: m.hypothetical ? ('square' as const) : ('circle' as const),
          color: m.hypothetical ? SIGNAL : '#6b7280',
          text: m.text,
          size,
        };
      })
      .sort((a, b) => (a.time as number) - (b.time as number));
    markerLayer.current.setMarkers(placed);
    // Only the first bar and the bar length matter here, not every tick.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [markers, bars[0]?.time, barSeconds]);

  // Price levels (entry and stop of open positions).
  useEffect(() => {
    const series = candles.current;
    if (!series) {
      return;
    }
    priceLines.current.forEach(line => series.removePriceLine(line));
    priceLines.current = levels.map(level =>
      series.createPriceLine({
        price: level.price,
        color: level.color,
        lineWidth: 1,
        lineStyle: level.dashed ? LineStyle.Dashed : LineStyle.Solid,
        axisLabelVisible: true,
        title: level.label,
      })
    );
  }, [levels]);

  return <div ref={container} style={{ height }} className="w-full" />;
}
