'use client'

import { useState } from 'react'
import { signIn } from 'next-auth/react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Shield, Lock, Smartphone } from 'lucide-react'

export default function SignInPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [showTotp, setShowTotp] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const router = useRouter()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      const result = await signIn('credentials', {
        email,
        password,
        totp: showTotp ? totp : undefined,
        redirect: false,
      })

      if (result?.error) {
        setError('Invalid credentials. Please try again.')
        return
      }

      if (result?.ok) {
        router.push('/dashboard')
      }
    } catch (error) {
      setError('An error occurred during sign in.')
    } finally {
      setLoading(false)
    }
  }

  const handleFirstStep = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      const result = await signIn('credentials', {
        email,
        password,
        redirect: false,
      })

      if (result?.error) {
        setError('Invalid email or password.')
        return
      }

      // For demo purposes, show TOTP field
      setShowTotp(true)
    } catch (error) {
      setError('An error occurred during sign in.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div className="text-center">
          <Shield className="mx-auto h-12 w-12 text-blue-600" />
          <h2 className="mt-6 text-3xl font-extrabold text-gray-900">
            Sign in to your account
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            Access your trading dashboard with secure authentication
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Authentication Required</CardTitle>
            <CardDescription>
              {showTotp 
                ? 'Enter your 2FA code to complete sign in'
                : 'Enter your credentials to continue'
              }
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={showTotp ? handleSubmit : handleFirstStep} className="space-y-6">
              {!showTotp && (
                <>
                  <div>
                    <Label htmlFor="email">Email address</Label>
                    <Input
                      id="email"
                      name="email"
                      type="email"
                      autoComplete="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="admin@example.com"
                    />
                  </div>

                  <div>
                    <Label htmlFor="password">Password</Label>
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      autoComplete="current-password"
                      required
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      placeholder="Enter your password"
                    />
                  </div>
                </>
              )}

              {showTotp && (
                <div>
                  <Label htmlFor="totp">2FA Code</Label>
                  <div className="flex items-center gap-2">
                    <Smartphone className="w-4 h-4 text-muted-foreground" />
                    <Input
                      id="totp"
                      name="totp"
                      type="text"
                      autoComplete="one-time-code"
                      required
                      value={totp}
                      onChange={(e) => setTotp(e.target.value)}
                      placeholder="123456"
                      maxLength={6}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground mt-1">
                    Demo: Use code <code className="bg-muted px-1 rounded">123456</code>
                  </p>
                </div>
              )}

              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <div className="flex gap-2">
                <Button
                  type="submit"
                  className="flex-1"
                  disabled={loading}
                >
                  {loading ? 'Signing in...' : showTotp ? 'Complete Sign In' : 'Continue'}
                </Button>
                {showTotp && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setShowTotp(false)}
                    disabled={loading}
                  >
                    Back
                  </Button>
                )}
              </div>
            </form>

            <div className="mt-6">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-300" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-2 bg-white text-gray-500">Demo Credentials</span>
                </div>
              </div>
              <div className="mt-4 text-xs text-muted-foreground space-y-1">
                <p><strong>Email:</strong> admin@example.com</p>
                <p><strong>Password:</strong> password123</p>
                <p><strong>2FA Code:</strong> 123456</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}