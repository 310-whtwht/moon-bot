import { auth } from '@/auth';

export default auth(req => {
  const isLoggedIn = !!req.auth;
  const { nextUrl } = req;

  // Protect all routes except auth pages
  if (!isLoggedIn && !nextUrl.pathname.startsWith('/auth')) {
    return Response.redirect(new URL('/auth/signin', nextUrl));
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
