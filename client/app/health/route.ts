// Local-dev only health proxy.
// In production, Traefik routes /health directly to the Go server.

import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(): Promise<NextResponse> {
	const apiUrl = process.env.API_URL;
	if (!apiUrl) {
		return NextResponse.json({ ok: false, message: "not found" }, { status: 404 });
	}

	const upstream = await fetch(`${apiUrl}/health`);
	const body = await upstream.text();
	return new NextResponse(body, {
		status: upstream.status,
		headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
	});
}
