"use client";

import { useState } from "react";
import { SubscribeModal } from "@/components/subscribe/subscribe-modal";
import { useSession } from "@/lib/session";

export function SubscribeCTA({ className, children }: { className?: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  const { user, loading } = useSession();

  if (!loading && user) return null;

  return (
    <>
      <button type="button" className={`cursor-pointer ${className ?? ""}`} onClick={() => setOpen(true)}>
        {children}
      </button>
      <SubscribeModal open={open} onOpenChange={setOpen} />
    </>
  );
}
