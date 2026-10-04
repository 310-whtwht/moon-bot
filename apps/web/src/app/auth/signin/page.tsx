'use client';

import { useState } from 'react';
import { signIn } from 'next-auth/react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Shield, Smartphone } from 'lucide-react';

export default function SignInPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [totp, setTotp] = useState('');
  const [showTotp, setShowTotp] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const router = useRouter();

  // One handler for both steps: the server answers "totp_required" when the
  // password is correct and a 2FA code is still needed.
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const result = await signIn('credentials', {
        email,
        password,
        totp: showTotp ? totp : '',
        redirect: false,
      });

      if (result?.code === 'totp_required') {
        setShowTotp(true);
        return;
      }
      if (result?.code === 'invalid_totp') {
        setError('2FA コードが正しくないか、有効期限が切れています。');
        return;
      }
      if (result?.error) {
        setError('メールアドレスまたはパスワードが正しくありません。');
        setShowTotp(false);
        return;
      }

      if (result?.ok) {
        router.push('/dashboard');
      }
    } catch {
      setError('ログイン中にエラーが発生しました。');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div className="text-center">
          <Shield className="mx-auto h-12 w-12 text-blue-600" />
          <h2 className="mt-6 text-3xl font-extrabold text-gray-900">
            ログイン
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            トレーディングダッシュボードにログインします
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>認証</CardTitle>
            <CardDescription>
              {showTotp
                ? '認証アプリに表示されている6桁のコードを入力してください'
                : 'メールアドレスとパスワードを入力してください'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-6">
              {!showTotp && (
                <>
                  <div>
                    <Label htmlFor="email">メールアドレス</Label>
                    <Input
                      id="email"
                      name="email"
                      type="email"
                      autoComplete="email"
                      required
                      value={email}
                      onChange={e => setEmail(e.target.value)}
                      placeholder="you@example.com"
                    />
                  </div>

                  <div>
                    <Label htmlFor="password">パスワード</Label>
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      autoComplete="current-password"
                      required
                      value={password}
                      onChange={e => setPassword(e.target.value)}
                      placeholder="パスワード"
                    />
                  </div>
                </>
              )}

              {showTotp && (
                <div>
                  <Label htmlFor="totp">2FA コード</Label>
                  <div className="flex items-center gap-2">
                    <Smartphone className="w-4 h-4 text-muted-foreground" />
                    <Input
                      id="totp"
                      name="totp"
                      type="text"
                      autoComplete="one-time-code"
                      required
                      value={totp}
                      onChange={e => setTotp(e.target.value)}
                      placeholder="000000"
                      inputMode="numeric"
                      maxLength={6}
                      autoFocus
                    />
                  </div>
                </div>
              )}

              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <div className="flex gap-2">
                <Button type="submit" className="flex-1" disabled={loading}>
                  {loading ? '確認中...' : showTotp ? 'ログイン' : '次へ'}
                </Button>
                {showTotp && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setShowTotp(false);
                      setTotp('');
                    }}
                    disabled={loading}
                  >
                    戻る
                  </Button>
                )}
              </div>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
