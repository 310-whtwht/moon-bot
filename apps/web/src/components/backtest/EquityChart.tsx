'use client';

import type { EquityPoint } from '@/lib/backtest';
import { formatYen } from '@/lib/backtest';

interface Props {
  points: EquityPoint[];
  initialBalance: number;
  height?: number;
}

/** Minimal SVG equity curve with the initial balance as a baseline. */
export function EquityChart({ points, initialBalance, height = 220 }: Props) {
  if (points.length < 2) {
    return (
      <p className="text-sm text-muted-foreground">
        表示できるデータがありません
      </p>
    );
  }

  const width = 800;
  const pad = 8;
  const values = points.map(p => p.equity);
  const min = Math.min(initialBalance, ...values);
  const max = Math.max(initialBalance, ...values);
  const span = max - min || 1;
  const x = (i: number) => pad + (i / (points.length - 1)) * (width - pad * 2);
  const y = (v: number) => pad + (1 - (v - min) / span) * (height - pad * 2);

  const path = points
    .map(
      (p, i) =>
        `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(p.equity).toFixed(1)}`
    )
    .join(' ');
  const last = points[points.length - 1]!;
  const up = last.equity >= initialBalance;

  return (
    <div>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full h-auto"
        role="img"
        aria-label="損益曲線"
      >
        <line
          x1={pad}
          x2={width - pad}
          y1={y(initialBalance)}
          y2={y(initialBalance)}
          stroke="currentColor"
          strokeOpacity={0.25}
          strokeDasharray="4 4"
        />
        <path
          d={path}
          fill="none"
          stroke={up ? '#16a34a' : '#dc2626'}
          strokeWidth={2}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="flex justify-between text-xs text-muted-foreground mt-1">
        <span>{new Date(points[0]!.time).toLocaleDateString('ja-JP')}</span>
        <span>
          最小 {formatYen(min)} / 最大 {formatYen(max)}
        </span>
        <span>{new Date(last.time).toLocaleDateString('ja-JP')}</span>
      </div>
    </div>
  );
}
