import { NextResponse } from 'next/server';
import { auth } from '@/auth';

const API_PREFIX = '/api/v1/';

/**
 * Forwards /api/v1/* to the Go API. The browser never talks to the API
 * directly: this adds the bearer token the API requires (server-side only),
 * and drops whatever Authorization header the client sent.
 */
function proxyToApi(req: Request, pathname: string, search: string) {
  const apiUrl = process.env.API_URL ?? 'http://localhost:8081';
  const token = process.env.API_TOKEN;

  const headers = new Headers(req.headers);
  headers.delete('authorization');
  if (token) {
    headers.set('authorization', `Bearer ${token}`);
  } else if (process.env.NODE_ENV !== 'development') {
    // Never forward unauthenticated in production: fail loudly instead.
    return NextResponse.json(
      { error: 'API_TOKEN is not configured on the web server' },
      { status: 500 }
    );
  }

  return NextResponse.rewrite(new URL(pathname + search, apiUrl), {
    request: { headers },
  });
}

export default auth(req => {
  const { nextUrl } = req;
  const isApi = nextUrl.pathname.startsWith(API_PREFIX);
  // Authentication is skipped in development mode (the API proxy still runs).
  const isDev = process.env.NODE_ENV === 'development';
  const isLoggedIn = !!req.auth;

  if (isApi) {
    if (!isDev && !isLoggedIn) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }
    return proxyToApi(req, nextUrl.pathname, nextUrl.search);
  }

  if (isDev) {
    return null;
  }

  // Protect all routes except auth pages
  if (!isLoggedIn && !nextUrl.pathname.startsWith('/auth')) {
    return Response.redirect(new URL('/auth/signin', nextUrl));
  }

  // Signed-in users have no use for the sign-in page
  if (isLoggedIn && nextUrl.pathname === '/auth/signin') {
    return Response.redirect(new URL('/dashboard', nextUrl));
  }

  return null;
});

export const config = {
  // Everything except NextAuth's own routes, the mock API and static assets.
  // /api/v1/* is included: it must pass the session check above.
  matcher: ['/((?!api/auth|api/mock|_next/static|_next/image|favicon.ico).*)'],
};
