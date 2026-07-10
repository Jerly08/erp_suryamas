import { NextRequest, NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

export function GET(request: NextRequest) {
  const requestUrl = new URL(request.url);
  const redirectUrl = new URL('/api/v1/branding/icon', requestUrl.origin);
  const size = requestUrl.searchParams.get('size') || '192';

  redirectUrl.searchParams.set('size', size);
  redirectUrl.searchParams.set('source', 'favicon-route');
  redirectUrl.searchParams.set('v', Date.now().toString());

  const response = NextResponse.redirect(redirectUrl, 307);
  response.headers.set('Cache-Control', 'no-store, no-cache, max-age=0, must-revalidate');
  response.headers.set('Pragma', 'no-cache');
  response.headers.set('Expires', '0');
  return response;
}
