import { NextRequest, NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

export function GET(request: NextRequest) {
  const requestUrl = new URL(request.url);
  const size = requestUrl.searchParams.get('size') || '192';
  const redirectParams = new URLSearchParams({
    size,
    source: 'favicon-route',
    v: Date.now().toString(),
  });

  const response = new NextResponse(null, {
    status: 307,
    headers: {
      Location: `/api/v1/branding/icon?${redirectParams.toString()}`,
    },
  });
  response.headers.set('Cache-Control', 'no-store, no-cache, max-age=0, must-revalidate');
  response.headers.set('Pragma', 'no-cache');
  response.headers.set('Expires', '0');
  return response;
}
