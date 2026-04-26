/**
/execute is the landing page for the Execute / Don't Execute buttons
in the auto-execution confirmation email. The page is a server
component that POSTs the signed token to /api/execution/confirm with
the user's session cookie attached, then renders the resulting
success / decline / error state. No client-side JS for the happy
path — the round-trip happens server-side so the token never lands
in the browser's history or any third-party script's view.
*/

import { cookies, headers } from "next/headers";
import Link from "next/link";

type ConfirmResponse = {
  ok: boolean;
  message: string;
  decline?: boolean;
};

const SCHWAB_POSITIONS_URL = "https://client.schwab.com/app/accounts/positions/#/";

async function postConfirm(token: string, action: string): Promise<ConfirmResponse> {
  const cookieStore = await cookies();
  const headerStore = await headers();
  const apiUrl = process.env.API_URL || "http://trading-server:8080";

  /**
  Forward the same vt_session cookie the browser sent so the upstream
  /api/execution/confirm handler sees the user as authenticated.
  */
  const sessionCookie = cookieStore.get("vt_session");
  const cookieHeader = sessionCookie ? `vt_session=${sessionCookie.value}` : "";

  /**
  Forwarded host header lets the upstream server understand the
  origin if it ever needs to log it; not strictly required for auth.
  */
  const host = headerStore.get("host") || "";

  try {
    const res = await fetch(`${apiUrl}/api/execution/confirm`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VT-Source": "dashboard",
        Cookie: cookieHeader,
        "X-Forwarded-Host": host,
      },
      body: JSON.stringify({ token, action }),
      cache: "no-store",
    });
    const json = (await res.json()) as ConfirmResponse;
    if (!json.ok && !json.message) {
      return { ok: false, message: `unexpected ${res.status}` };
    }
    return json;
  } catch (err) {
    return { ok: false, message: `network: ${err instanceof Error ? err.message : String(err)}` };
  }
}

export default async function ExecutePage({ searchParams }: { searchParams: Promise<{ token?: string; action?: string }> }) {
  const params = await searchParams;
  const token = params.token ?? "";
  const action = params.action ?? "";

  if (!token || !action) {
    return (
      <PageShell title="Invalid link">
        <p className="text-sm text-muted-foreground">
          This URL is missing the required token or action parameter. Make sure you clicked the button directly from the confirmation email and didn&apos;t copy a partial URL.
        </p>
      </PageShell>
    );
  }
  if (action !== "execute" && action !== "decline") {
    return (
      <PageShell title="Invalid action">
        <p className="text-sm text-muted-foreground">Action must be either &ldquo;execute&rdquo; or &ldquo;decline&rdquo;.</p>
      </PageShell>
    );
  }

  const result = await postConfirm(token, action);

  if (!result.ok) {
    return (
      <PageShell title="Could not confirm">
        <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4">
          <p className="text-sm font-medium text-destructive">{result.message}</p>
          <p className="mt-2 text-xs text-muted-foreground">
            Common causes: the 5-minute confirmation window has expired, the link was already used, or you aren&apos;t signed in. The trade was <strong>not</strong> executed.
          </p>
        </div>
        <div className="mt-6">
          <Link href="/dashboard" className="inline-flex min-h-11 items-center text-sm text-primary underline underline-offset-2">
            Back to dashboard
          </Link>
        </div>
      </PageShell>
    );
  }

  if (result.decline) {
    return (
      <PageShell title="Trade declined">
        <p className="text-sm text-muted-foreground">{result.message}</p>
        <div className="mt-6">
          <Link href="/dashboard" className="inline-flex min-h-11 items-center text-sm text-primary underline underline-offset-2">
            Back to dashboard
          </Link>
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell title="✓ Trade execution confirmed">
      <p className="text-sm text-muted-foreground">{result.message}</p>
      <div className="mt-4 rounded-lg border bg-muted/30 p-4">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Next</p>
        <ul className="mt-2 space-y-1 text-sm">
          <li>• You&apos;ll receive a fill receipt email shortly with the order ID and fill price.</li>
          <li>
            • The position will be auto-closed at <strong>3:55 PM ET</strong> regardless of P&amp;L.
          </li>
          <li>• A close receipt email arrives once the close fills.</li>
        </ul>
      </div>
      <div className="mt-6 flex flex-col gap-2 sm:flex-row">
        <a
          href={SCHWAB_POSITIONS_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
        >
          View on Schwab →
        </a>
        <Link href="/dashboard" className="inline-flex items-center justify-center rounded-md border px-4 py-2 text-sm font-medium hover:bg-accent">
          Back to dashboard
        </Link>
      </div>
    </PageShell>
  );
}

function PageShell({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="container mx-auto max-w-xl py-12">
      <h1 className="mb-4 text-2xl font-bold tracking-tight">{title}</h1>
      {children}
    </div>
  );
}
