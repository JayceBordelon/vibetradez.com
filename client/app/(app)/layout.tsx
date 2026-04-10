"use client";

import { useState } from "react";
import { Footer } from "@/components/layout/footer";
import { NavBar } from "@/components/layout/nav-bar";
import { TopBar } from "@/components/layout/top-bar";
import { SubscribeModal } from "@/components/subscribe/subscribe-modal";

export default function AppLayout({ children }: { children: React.ReactNode }) {
	const [modalOpen, setModalOpen] = useState(false);

	return (
		<div className="flex min-h-dvh flex-col">
			<TopBar onSubscribe={() => setModalOpen(true)} />
			<NavBar />
			<main className="flex-1">{children}</main>
			<Footer />
			<SubscribeModal open={modalOpen} onOpenChange={setModalOpen} />
		</div>
	);
}
