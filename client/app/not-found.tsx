import type { Metadata } from "next";

import { NotFoundContent } from "@/components/layout/not-found-content";

export const metadata: Metadata = {
  title: "404 - Page not found",
  description: "The page you were looking for doesn't exist on VibeTradez.",
};

// Fallback 404 for any top-level route that matches nothing (outside the
// app group). The CRT chrome comes from the root layout.
export default function NotFound() {
  return (
    <div className="relative min-h-dvh overflow-hidden bg-background text-foreground">
      <div className="relative z-10">
        <NotFoundContent />
      </div>
    </div>
  );
}
