"use client";

import { useState } from "react";
import { SubscribeModal } from "@/components/subscribe/subscribe-modal";

export function SubscribeCTA({ className, children }: { className?: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)}>
        {children}
      </button>
      <SubscribeModal open={open} onOpenChange={setOpen} />
    </>
  );
}
