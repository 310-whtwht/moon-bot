import { Spinner } from '@/components/ui/spinner';

/** Shown while the next page is being prepared. */
export default function Loading() {
  return (
    <div className="container mx-auto p-6">
      <Spinner className="h-64" />
    </div>
  );
}
