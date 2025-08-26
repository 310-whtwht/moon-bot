import { Button } from '@/components/ui/button';
import Link from 'next/link';

export default function Home() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-between p-24">
      <div className="z-10 max-w-5xl w-full items-center justify-between font-mono text-sm lg:flex">
        <div className="fixed bottom-0 left-0 flex h-48 w-full items-end justify-center bg-gradient-to-t from-white via-white dark:from-black dark:via-black lg:static lg:h-auto lg:w-auto lg:bg-none">
          <div className="flex place-items-center gap-2 p-8 lg:pointer-events-auto lg:p-0">
            <h1 className="text-4xl font-bold">Moomoo トレーディングシステム</h1>
          </div>
        </div>
      </div>

      <div className="relative flex place-items-center before:absolute before:h-[300px] before:w-[480px] before:-translate-x-1/2 before:-translate-y-1/2 before:rounded-full before:bg-gradient-radial before:from-white before:to-transparent before:blur-2xl before:content-[''] after:absolute after:-z-20 after:h-[180px] after:w-[240px] after:translate-x-1/3 after:bg-gradient-conic after:from-sky-200 after:via-blue-200 after:blur-2xl after:content-[''] before:dark:bg-gradient-to-br before:dark:from-transparent before:dark:to-blue-700 before:dark:opacity-10 after:dark:from-sky-900 after:dark:via-[#0141ff] after:dark:opacity-40 before:lg:h-[360px] z-[-1]">
        <div className="text-center">
          <h2 className="text-2xl font-semibold mb-4">
            Moomoo トレーディングへようこそ
          </h2>
          <p className="text-lg mb-8">高度なアルゴリズム取引プラットフォーム</p>
          <div className="space-x-4">
            <Link href="/dashboard">
              <Button>ダッシュボード</Button>
            </Link>
            <Link href="/strategies">
              <Button variant="outline">始める</Button>
            </Link>
          </div>
        </div>
      </div>

      <div className="mb-32 grid text-center lg:max-w-5xl lg:w-full lg:mb-0 lg:grid-cols-4 lg:text-left">
        <Link href="/strategies">
          <div className="group rounded-lg border border-transparent px-5 py-4 transition-colors hover:border-gray-300 hover:bg-gray-100 hover:dark:border-neutral-700 hover:dark:bg-neutral-800/30">
            <h3 className="mb-3 text-2xl font-semibold">
              戦略管理{' '}
              <span className="inline-block transition-transform group-hover:translate-x-1 motion-reduce:transform-none">
                →
              </span>
            </h3>
            <p className="m-0 max-w-[30ch] text-sm opacity-50">
              Starlarkスクリプトを使用したトレーディング戦略の作成と管理
            </p>
          </div>
        </Link>

        <Link href="/backtests">
          <div className="group rounded-lg border border-transparent px-5 py-4 transition-colors hover:border-gray-300 hover:bg-gray-100 hover:dark:border-neutral-700 hover:dark:bg-neutral-800/30">
            <h3 className="mb-3 text-2xl font-semibold">
              バックテスト{' '}
              <span className="inline-block transition-transform group-hover:translate-x-1 motion-reduce:transform-none">
                →
              </span>
            </h3>
            <p className="m-0 max-w-[30ch] text-sm opacity-50">
              履歴データとパフォーマンス指標を使用した戦略のテスト
            </p>
          </div>
        </Link>

        <Link href="/orders">
          <div className="group rounded-lg border border-transparent px-5 py-4 transition-colors hover:border-gray-300 hover:bg-gray-100 hover:dark:border-neutral-700 hover:dark:bg-neutral-800/30">
            <h3 className="mb-3 text-2xl font-semibold">
              注文・取引{' '}
              <span className="inline-block transition-transform group-hover:translate-x-1 motion-reduce:transform-none">
                →
              </span>
            </h3>
            <p className="m-0 max-w-[30ch] text-sm opacity-50">
              リアルタイム実行追跡による注文と取引の監視
            </p>
          </div>
        </Link>

        <Link href="/universe">
          <div className="group rounded-lg border border-transparent px-5 py-4 transition-colors hover:border-gray-300 hover:bg-gray-100 hover:dark:border-neutral-700 hover:dark:bg-neutral-800/30">
            <h3 className="mb-3 text-2xl font-semibold">
              銘柄管理{' '}
              <span className="inline-block transition-transform group-hover:translate-x-1 motion-reduce:transform-none">
                →
              </span>
            </h3>
            <p className="m-0 max-w-[30ch] text-sm opacity-50">
              取引可能な銘柄とアセットユニバース設定の管理
            </p>
          </div>
        </Link>

        <Link href="/notifications">
          <div className="group rounded-lg border border-transparent px-5 py-4 transition-colors hover:border-gray-300 hover:bg-gray-100 hover:dark:border-neutral-700 hover:dark:bg-neutral-800/30">
            <h3 className="mb-3 text-2xl font-semibold">
              通知{' '}
              <span className="inline-block transition-transform group-hover:translate-x-1 motion-reduce:transform-none">
                →
              </span>
            </h3>
            <p className="m-0 max-w-[30ch] text-sm opacity-50">
              システムアラートと通知設定の管理
            </p>
          </div>
        </Link>
      </div>
    </main>
  );
}
