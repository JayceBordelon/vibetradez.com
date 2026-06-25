"use client";

import { useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { SubscribeForm, UnsubscribeForm } from "./subscribe-form";

export function SubscribeModal({ children, open, onOpenChange }: { children?: React.ReactNode; open?: boolean; onOpenChange?: (open: boolean) => void }) {
  const [showUnsub, setShowUnsub] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {children && <DialogTrigger asChild>{children}</DialogTrigger>}
      <DialogContent className="lg-modal top-[8%] max-h-[92dvh] w-[calc(100%-1.5rem)] max-w-[calc(100%-1.5rem)] translate-y-0 overflow-y-auto sm:top-[50%] sm:w-auto sm:max-w-md sm:translate-y-[-50%]">
        <DialogHeader>
          <DialogTitle className="font-display text-xl font-bold tracking-tight">
            Sign in or <span className="text-primary">sign up</span>
          </DialogTitle>
          <DialogDescription>One click with Google gets you the recap email each session the model trades.</DialogDescription>
        </DialogHeader>

        <div className="flex gap-2.5">
          {[
            { v: "1", l: "Session recap" },
            { v: "16:00", l: "After the close" },
            { v: "$0", l: "Always free" },
          ].map((s) => (
            <div key={s.l} className="flex-1 rounded-xl border border-border bg-muted/50 p-3 text-center">
              <div className="text-lg font-bold tracking-tight text-primary tabular-nums">{s.v}</div>
              <div className="mt-0.5 text-[10px] font-medium text-muted-foreground">{s.l}</div>
            </div>
          ))}
        </div>

        <SubscribeForm />

        <div className="border-t pt-4 text-center">
          <button type="button" onClick={() => setShowUnsub(!showUnsub)} className="cursor-pointer text-[11px] text-muted-foreground underline underline-offset-2 hover:text-foreground">
            Need to unsubscribe?
          </button>
          {showUnsub && (
            <div className="mt-3">
              <UnsubscribeForm />
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
