"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { signInWithGoogle, useSession } from "@/lib/session";

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={className}>
      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.56c2.08-1.92 3.28-4.74 3.28-8.1z" />
      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.65l-3.56-2.77c-.99.66-2.25 1.05-3.72 1.05-2.86 0-5.29-1.93-6.15-4.53H2.18v2.84A11 11 0 0 0 12 23z" />
      <path fill="#FBBC05" d="M5.85 14.1A6.6 6.6 0 0 1 5.5 12c0-.73.13-1.44.35-2.1V7.07H2.18A11 11 0 0 0 1 12c0 1.78.43 3.47 1.18 4.94l3.67-2.84z" />
      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.2 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.07l3.67 2.84C6.71 7.31 9.14 5.38 12 5.38z" />
    </svg>
  );
}

export function SubscribeForm() {
  const { user, loading, signOut } = useSession();

  if (loading) {
    return <div className="h-10 rounded-md bg-muted/60" aria-hidden="true" />;
  }

  if (user) {
    return (
      <div className="space-y-2.5">
        <p className="rounded-md bg-green-bg p-3 text-xs font-semibold text-green">You're subscribed as {user.email}. The next morning's picks will land in your inbox before the opening bell.</p>
        <button type="button" onClick={signOut} className="cursor-pointer text-[11px] text-muted-foreground underline underline-offset-2 hover:text-foreground">
          Not you? Sign out
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-2.5">
      <Button type="button" className="w-full gap-2" onClick={() => signInWithGoogle()}>
        <GoogleIcon className="h-4 w-4" />
        Continue with Google
      </Button>
      <p className="text-center text-[11px] text-muted-foreground">Signing in subscribes you to the daily picks email. You can unsubscribe anytime.</p>
    </div>
  );
}

export function UnsubscribeForm() {
  const [email, setEmail] = useState("");
  const [status, setStatus] = useState<"idle" | "loading" | "success" | "error">("idle");
  const [message, setMessage] = useState("");

  async function handleSubmit() {
    if (!email.trim()) return;
    setStatus("loading");
    try {
      const res = await api.unsubscribe(email.trim());
      if (res.ok) {
        setStatus("success");
        setMessage("You've been unsubscribed. Sorry to see you go!");
        setEmail("");
      } else {
        setStatus("error");
        setMessage(res.message || "Something went wrong.");
      }
    } catch {
      setStatus("error");
      setMessage("Connection error.");
    }
  }

  return (
    <div className="space-y-3">
      <Input type="email" placeholder="your@email.com" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
      <Button variant="outline" className="w-full text-red" disabled={status === "loading"} onClick={handleSubmit}>
        {status === "loading" ? "Unsubscribing..." : "Unsubscribe"}
      </Button>
      {status === "success" && <p className="rounded-md bg-green-bg p-2.5 text-xs font-semibold text-green">{message}</p>}
      {status === "error" && <p className="rounded-md bg-red-bg p-2.5 text-xs font-semibold text-red">{message}</p>}
    </div>
  );
}
