import TraceVisualizer from '@/components/audit/TraceVisualizer'

export default function AuditPage() {
  return (
    <div className="container mx-auto py-6">
      <div className="mb-6">
        <h1 className="text-3xl font-bold">監査・ログ</h1>
        <p className="text-gray-600 mt-2">
          トレードの鎖状トレースと監査ログを確認できます
        </p>
      </div>
      
      <TraceVisualizer />
    </div>
  )
}