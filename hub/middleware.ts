import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Routes that require the user to be authenticated
const PROTECTED_ROUTES = ["/registry/submit", "/feed"];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Check for Supabase auth token cookies (set by @supabase/ssr)
  // The anon browser client sets cookies prefixed with sb-
  const hasAuthToken =
    request.cookies.has("sb-access-token") ||
    // Supabase v2 SSR stores session in a chunked cookie
    request.cookies.getAll().some(
      (cookie) => cookie.name.startsWith("sb-") && cookie.name.endsWith("-auth-token")
    );

  const isProtected = PROTECTED_ROUTES.some(
    (route) => pathname === route || pathname.startsWith(route + "/")
  );

  // Redirect unauthenticated users away from protected routes
  if (isProtected && !hasAuthToken) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("next", pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    // Apply to all routes except static files, images, and Next.js internals
    "/((?!_next/static|_next/image|favicon.ico|images/|icons/).*)",
  ],
};
