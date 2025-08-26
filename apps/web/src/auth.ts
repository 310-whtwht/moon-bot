import NextAuth from 'next-auth';
import type { NextAuthConfig } from 'next-auth';
import CredentialsProvider from 'next-auth/providers/credentials';
import { compare } from 'bcryptjs';

interface User {
  id: string;
  email: string;
  name: string;
  role: string;
}

const authConfig: NextAuthConfig = {
  providers: [
    CredentialsProvider({
      name: 'credentials',
      credentials: {
        email: { label: 'Email', type: 'email' },
        password: { label: 'Password', type: 'password' },
        totp: { label: 'TOTP Code', type: 'text' },
      },
      async authorize(credentials) {
        // In development mode, always authenticate successfully
        if (process.env.NODE_ENV === 'development') {
          return {
            id: 'dev-user',
            email: (credentials?.email as string) || 'dev@example.com',
            name: 'Development User',
            role: 'admin',
          };
        }

        if (!credentials?.email || !credentials?.password) {
          return null;
        }

        try {
          // Mock user authentication - replace with actual database lookup
          const mockUser = {
            id: '1',
            email: 'admin@example.com',
            password: '$2a$10$example.hash', // This should be the actual hashed password
            name: 'Admin User',
            role: 'admin',
          };

          if (credentials.email === mockUser.email) {
            const isValidPassword = await compare(
              credentials.password as string,
              mockUser.password as string
            );
            if (isValidPassword) {
              return {
                id: mockUser.id,
                email: mockUser.email,
                name: mockUser.name,
                role: mockUser.role,
              };
            }
          }

          return null;
        } catch (error) {
          console.error('Authentication error:', error);
          return null;
        }
      },
    }),
  ],
  callbacks: {
    async jwt({ token, user }) {
      if (user) {
        token.role = (user as User).role;
      }
      return token;
    },
    async session({ session, token }) {
      if (token.sub) {
        session.user.id = token.sub;
      }
      if (token.role) {
        (session.user as { role?: string }).role = token.role as string;
      }
      return session;
    },
  },
  pages: {
    signIn: '/auth/signin',
    error: '/auth/error',
  },
  session: {
    strategy: 'jwt',
  },
};

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);
