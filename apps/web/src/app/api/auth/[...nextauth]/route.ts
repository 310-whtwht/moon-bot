import NextAuth from 'next-auth'
import { NextAuthOptions } from 'next-auth'
import CredentialsProvider from 'next-auth/providers/credentials'
import { compare } from 'bcryptjs'
import { JWT } from 'next-auth/jwt'
import { Session } from 'next-auth'

interface Credentials {
  email: string
  password: string
  totp?: string
}

interface User {
  id: string
  email: string
  name: string
  role: string
}

interface ExtendedToken extends JWT {
  role?: string
}

interface ExtendedSession extends Session {
  user: {
    id?: string
    role?: string
  } & Session['user']
}

export const authOptions: NextAuthOptions = {
  providers: [
    CredentialsProvider({
      name: 'credentials',
      credentials: {
        email: { label: 'Email', type: 'email' },
        password: { label: 'Password', type: 'password' },
        totp: { label: '2FA Code', type: 'text' }
      },
      async authorize(credentials: Credentials | undefined) {
        if (!credentials?.email || !credentials?.password) {
          return null
        }

        try {
          // TODO: Replace with actual database lookup
          // For now, using mock user data
          const mockUser = {
            id: '1',
            email: 'admin@example.com',
            name: 'Admin User',
            password: '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4tbQJELpKi', // 'password123'
            totpSecret: 'JBSWY3DPEHPK3PXP', // Mock TOTP secret
            role: 'admin'
          }

          if (credentials.email !== mockUser.email) {
            return null
          }

          const isValidPassword = await compare(credentials.password, mockUser.password)
          if (!isValidPassword) {
            return null
          }

          // TODO: Implement TOTP verification
          // For now, skip 2FA for development
          if (credentials.totp && credentials.totp !== '123456') {
            return null
          }

          return {
            id: mockUser.id,
            email: mockUser.email,
            name: mockUser.name,
            role: mockUser.role
          }
        } catch (error) {
          console.error('Auth error:', error)
          return null
        }
      }
    })
  ],
  session: {
    strategy: 'jwt',
    maxAge: 24 * 60 * 60, // 24 hours
  },
  callbacks: {
    async jwt({ token, user }: { token: ExtendedToken; user?: User }) {
      if (user) {
        token.role = user.role
      }
      return token
    },
    async session({ session, token }: { session: ExtendedSession; token: ExtendedToken }) {
      if (token && session.user) {
        session.user.id = token.sub
        session.user.role = token.role
      }
      return session
    }
  },
  pages: {
    signIn: '/auth/signin',
    error: '/auth/error',
  },
  secret: process.env.NEXTAUTH_SECRET || 'your-secret-key',
  // Docker環境での認証設定
  trustHost: true,
  // 開発環境での設定
  debug: process.env.NODE_ENV === 'development',
}

const handler = NextAuth(authOptions)

export { handler as GET, handler as POST }