/**
Catch-all auth proxy for local development.

In production, Traefik routes /auth/* to the Go server before the request
reaches Next.js. This handler only fires in dev (where API_URL is set).

Two things make this different from the /api/* proxy:
- redirect: "manual" so the 302 from the Go OAuth callback flows back to
  the browser instead of being followed server-side (which would resolve
  to an unreachable google.com from the Docker network).
- getSetCookie() to preserve multiple Set-Cookie headers; the session
  cookie and the oauth_state clear-cookie both ship in the same response.
*/

import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

async function proxy(req: NextRequest): Promise<NextResponse> {
  const apiUrl = process.env.API_URL;
  if (!apiUrl) {
    return NextResponse.json({ ok: false, message: "not found" }, { status: 404 });
  }

  const url = new URL(req.url);
  const target = `${apiUrl}${url.pathname}${url.search}`;

  const headers = new Headers(req.headers);
  headers.delete("host");
  headers.delete("connection");
  headers.delete("content-length");

  const init: RequestInit = {
    method: req.method,
    headers,
    redirect: "manual",
  };

  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }

  const upstream = await fetch(target, init);
  const responseHeaders = new Headers();
  for (const [key, value] of upstream.headers.entries()) {
    if (key === "set-cookie" || key === "content-encoding" || key === "transfer-encoding") continue;
    responseHeaders.set(key, value);
  }
  for (const cookie of upstream.headers.getSetCookie()) {
    responseHeaders.append("set-cookie", cookie);
  }

  return new NextResponse(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: responseHeaders,
  });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const OPTIONS = proxy;
export const HEAD = proxy;
