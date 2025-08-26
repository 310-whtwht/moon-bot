'use client'

import { useState } from 'react'
import { signIn, getSession } from 'next-auth/react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function TestLoginPage() {
  const [email, setEmail] = useState('admin@example.com')
  const [password, setPassword] = useState('password123')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const router = useRouter()

  const handleTestLogin = async () => {
    setLoading(true)
    setError('')
    setSuccess('')

    try {
      console.log('Attempting test login with:', { email, password })
      
      const result = await signIn('credentials', {
        email,
        password,
        redirect: false,
      })

      console.log('Sign in result:', result)

      if (result?.error) {
        setError(`Login failed: ${result.error}`)
        return
      }

      if (result?.ok) {
        setSuccess('Login successful! Redirecting to dashboard...')
        setTimeout(() => {
          router.push('/dashboard')
        }, 1000)
      }
    } catch (error) {
      console.error('Test login error:', error)
      setError(`An error occurred: ${error instanceof Error ? error.message : 'Unknown error'}`)
    } finally {
      setLoading(false)
    }
  }

  const handleCheckSession = async () => {
    try {
      const session = await getSession()
      console.log('Current session:', session)
      if (session) {
        setSuccess(`Logged in as: ${session.user?.email}`)
      } else {
        setError('No active session found')
      }
    } catch (error) {
      setError(`Session check failed: ${error instanceof Error ? error.message : 'Unknown error'}`)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div className="text-center">
          <h2 className="mt-6 text-3xl font-extrabold text-gray-900">
            Test Login
          </h2>
          <p className="mt-2 text-sm text-gray-600">
            Quick test login with predefined credentials
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Test Authentication</CardTitle>
            <CardDescription>
              Use the test credentials to verify authentication is working
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div>
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="admin@example.com"
              />
            </div>

            <div>
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="password123"
              />
            </div>

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {success && (
              <Alert>
                <AlertDescription>{success}</AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Button
                onClick={handleTestLogin}
                disabled={loading}
                className="w-full"
              >
                {loading ? 'Testing Login...' : 'Test Login'}
              </Button>
              
              <Button
                onClick={handleCheckSession}
                variant="outline"
                className="w-full"
              >
                Check Session
              </Button>
            </div>

            <div className="text-xs text-muted-foreground space-y-1">
              <p><strong>Test Credentials:</strong></p>
              <p>Email: admin@example.com</p>
              <p>Password: password123</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}