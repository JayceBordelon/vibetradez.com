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
	const responseHeaders = new Headers(upstream.headers);
	responseHeaders.delete("content-encoding");
	responseHeaders.delete("transfer-encoding");

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
