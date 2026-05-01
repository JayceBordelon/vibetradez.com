"use client";

import { useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { SubscribeForm, UnsubscribeForm } from "./subscribe-form";

export function SubscribeModal({ children, open, onOpenChange }: { children?: React.ReactNode; open?: boolean; onOpenChange?: (open: boolean) => void }) {
  const [showUnsub, setShowUnsub] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {children && <DialogTrigger asChild>{children}</DialogTrigger>}
      <DialogContent className="lg-card top-[8%] max-h-[92dvh] w-[calc(100%-1.5rem)] max-w-[calc(100%-1.5rem)] translate-y-0 overflow-y-auto sm:top-[50%] sm:w-auto sm:max-w-md sm:translate-y-[-50%]">
        <DialogHeader>
          <DialogTitle className="text-lg font-extrabold">
            Sign in or <span className="text-gradient-brand">sign up</span>
          </DialogTitle>
          <DialogDescription>One click with Google gets you the daily picks email, delivered before the opening bell.</DialogDescription>
        </DialogHeader>

        <div className="flex gap-2">
          <div className="flex-1 rounded-lg border bg-muted p-2.5 text-center">
            <div className="font-mono text-lg font-extrabold text-primary">2</div>
            <div className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">AI Pickers</div>
          </div>
          <div className="flex-1 rounded-lg border bg-muted p-2.5 text-center">
            <div className="font-mono text-lg font-extrabold text-primary">2x</div>
            <div className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">AM + EOD</div>
          </div>
          <div className="flex-1 rounded-lg border bg-muted p-2.5 text-center">
            <div className="font-mono text-lg font-extrabold text-primary">$0</div>
            <div className="text-[9px] font-semibold uppercase tracking-wide text-muted-foreground">Always Free</div>
          </div>
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
