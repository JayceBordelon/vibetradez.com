"use client";

import { useState } from "react";
import { AmbientBackground } from "@/components/landing/ambient-background";
import { Footer } from "@/components/layout/footer";
import { NavBar } from "@/components/layout/nav-bar";
import { SubscribeModal } from "@/components/subscribe/subscribe-modal";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [modalOpen, setModalOpen] = useState(false);

  return (
    <div className="relative flex min-h-dvh flex-col overflow-x-hidden bg-background text-foreground">
      <AmbientBackground density="muted" />
      <div className="relative z-10 flex min-h-dvh flex-col">
        <NavBar onSubscribe={() => setModalOpen(true)} />
        <main className="flex-1">{children}</main>
        <Footer />
      </div>
      <SubscribeModal open={modalOpen} onOpenChange={setModalOpen} />
    </div>
  );
}
