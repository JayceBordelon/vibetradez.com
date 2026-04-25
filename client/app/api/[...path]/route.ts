// Catch-all API proxy.
//
// In production, Traefik routes /api/* to the Go server before requests
// ever reach Next.js, so this handler never fires. It only handles requests
// in local development (where there is no reverse proxy) by forwarding to
// the Go server identified by the API_URL env var.
//
// If API_URL isn't set, returns 404 (matches the previous Next.js behavior).

import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

async function proxy(req: NextRequest): Promise<NextResponse> {
  const apiUrl = process.env.API_URL;
  if (!apiUrl) {
    return NextResponse.json({ ok: false, message: "not found" }, { status: 404 });
  }

  let target: string;
  try {
    const url = new URL(req.url);
    target = `${apiUrl}${url.pathname}${url.search}`;
  } catch (e) {
    console.error("[api-proxy] bad request url", req.url, e);
    return NextResponse.json({ ok: false, message: "bad request" }, { status: 400 });
  }

  // Build a fresh, minimal header set. Forwarding the entire Headers object
  // sometimes drags in hop-by-hop headers (`connection`, `transfer-encoding`)
  // that the upstream fetch refuses to send and that cause the whole request
  // to throw before it ever leaves the container.
  const upstreamHeaders = new Headers();
  for (const [k, v] of req.headers.entries()) {
    const lower = k.toLowerCase();
    if (
      lower === "host" ||
      lower === "connection" ||
      lower === "content-length" ||
      lower === "transfer-encoding" ||
      lower === "keep-alive" ||
      lower === "te" ||
      lower === "upgrade" ||
      lower === "proxy-connection"
    ) {
      continue;
    }
    upstreamHeaders.set(k, v);
  }

  const init: RequestInit = {
    method: req.method,
    headers: upstreamHeaders,
    redirect: "manual",
  };

  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }

  let upstream: Response;
  try {
    upstream = await fetch(target, init);
  } catch (e) {
    console.error("[api-proxy] upstream fetch failed", target, e);
    return NextResponse.json({ ok: false, message: "upstream fetch failed" }, { status: 502 });
  }

  // Read the body as bytes once so we can set an explicit Content-Length
  // and avoid streaming-related quirks in the runtime's response writer.
  const body = await upstream.arrayBuffer();
  const responseHeaders = new Headers();
  for (const [k, v] of upstream.headers.entries()) {
    const lower = k.toLowerCase();
    if (lower === "content-encoding" || lower === "transfer-encoding" || lower === "connection" || lower === "keep-alive") {
      continue;
    }
    responseHeaders.set(k, v);
  }
  responseHeaders.set("content-length", String(body.byteLength));

  return new NextResponse(body, {
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
