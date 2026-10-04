import { auth } from '@/auth';

export default auth(req => {
  // Skip authentication in development mode
  if (process.env.NODE_ENV === 'development') {
    return null;
  }

  const isLoggedIn = !!req.auth;
  const { nextUrl } = req;

  // Protect all routes except auth pages
  if (!isLoggedIn && !nextUrl.pathname.startsWith('/auth')) {
    return Response.redirect(new URL('/auth/signin', nextUrl));
  }

  // Signed-in users have no use for the sign-in page
  if (isLoggedIn && nextUrl.pathname === '/auth/signin') {
    return Response.redirect(new URL('/dashboard', nextUrl));
  }

  // Allow access to auth pages
  if (nextUrl.pathname.startsWith('/auth')) {
    return null;
  }

  // Allow access to all other pages for authenticated users
  return null;
});

export const config = {
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico).*)'],
};
