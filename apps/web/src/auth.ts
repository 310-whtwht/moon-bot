import NextAuth, { CredentialsSignin } from 'next-auth';
import type { NextAuthConfig } from 'next-auth';
import CredentialsProvider from 'next-auth/providers/credentials';
import { compare } from 'bcryptjs';
import { verifyTotp } from '@/lib/totp';

interface User {
  id: string;
  email: string;
  name: string;
  role: string;
}

/** Password was correct but the 2FA code is still needed. */
class TotpRequired extends CredentialsSignin {
  code = 'totp_required';
}

/** The 2FA code was wrong or expired. */
class InvalidTotp extends CredentialsSignin {
  code = 'invalid_totp';
}

/**
 * Single admin user configured through environment variables:
 *   ADMIN_EMAIL          login email
 *   ADMIN_PASSWORD_HASH  bcrypt hash of the password (npm run auth:setup)
 *   ADMIN_TOTP_SECRET    base32 TOTP secret; when set, 2FA is required
 */
async function authorizeAdmin(
  credentials: Partial<Record<'email' | 'password' | 'totp', unknown>>
): Promise<User | null> {
  const email = process.env.ADMIN_EMAIL;
  const passwordHash = process.env.ADMIN_PASSWORD_HASH;
  const totpSecret = process.env.ADMIN_TOTP_SECRET;

  if (!email || !passwordHash) {
    console.error(
      '[auth] ADMIN_EMAIL / ADMIN_PASSWORD_HASH are not set; nobody can sign in'
    );
    return null;
  }

  const inputEmail = String(credentials.email ?? '')
    .trim()
    .toLowerCase();
  const inputPassword = String(credentials.password ?? '');
  if (inputEmail !== email.toLowerCase() || !inputPassword) {
    return null;
  }
  if (!(await compare(inputPassword, passwordHash))) {
    return null;
  }

  if (totpSecret) {
    const code = String(credentials.totp ?? '').trim();
    if (!code) {
      throw new TotpRequired();
    }
    if (!(await verifyTotp(totpSecret, code))) {
      throw new InvalidTotp();
    }
  }

  return { id: 'admin', email, name: 'Admin', role: 'admin' };
}

// Auth.js v5 reads AUTH_SECRET; NEXTAUTH_SECRET is accepted for existing deployments.
const secret = process.env.AUTH_SECRET ?? process.env.NEXTAUTH_SECRET;

const authConfig: NextAuthConfig = {
  ...(secret ? { secret } : {}),
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
          } as User;
        }

        return authorizeAdmin(credentials ?? {});
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
