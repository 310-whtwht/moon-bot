import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';

/** A spinning indicator with a label, for anything that is still loading. */
export function Spinner({
  label = '読み込み中...',
  className,
}: {
  label?: string;
  className?: string;
}) {
  return (
    <div
      role="status"
      className={cn(
        'flex items-center justify-center gap-3 text-muted-foreground',
        className
      )}
    >
      <Loader2 className="h-6 w-6 animate-spin" />
      <span>{label}</span>
    </div>
  );
}
