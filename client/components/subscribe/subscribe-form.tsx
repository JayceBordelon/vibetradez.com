"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export function SubscribeForm() {
	const [email, setEmail] = useState("");
	const [name, setName] = useState("");
	const [status, setStatus] = useState<"idle" | "loading" | "success" | "error">("idle");
	const [message, setMessage] = useState("");

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		if (!email.trim()) return;

		setStatus("loading");
		try {
			const res = await api.subscribe(email.trim(), name.trim());
			if (res.ok) {
				setStatus("success");
				setMessage("You're subscribed! First picks arrive before the next market open.");
				setEmail("");
				setName("");
			} else {
				setStatus("error");
				setMessage(res.message || "Something went wrong.");
			}
		} catch {
			setStatus("error");
			setMessage("Connection error. Please try again.");
		}
	}

	return (
		<form onSubmit={handleSubmit} className="space-y-3">
			<div>
				<Label htmlFor="sub-name" className="text-[11px] uppercase tracking-wide">
					Name (optional)
				</Label>
				<Input
					id="sub-name"
					placeholder="Your name"
					value={name}
					onChange={(e) => setName(e.target.value)}
					autoComplete="name"
				/>
			</div>
			<div>
				<Label htmlFor="sub-email" className="text-[11px] uppercase tracking-wide">
					Email
				</Label>
				<Input
					id="sub-email"
					type="email"
					placeholder="you@example.com"
					required
					value={email}
					onChange={(e) => setEmail(e.target.value)}
					autoComplete="email"
				/>
			</div>
			<Button type="submit" className="w-full" disabled={status === "loading"}>
				{status === "loading" ? "Subscribing..." : "Subscribe"}
			</Button>
			{status === "success" && (
				<p className="rounded-md bg-green-bg p-2.5 text-xs font-semibold text-green">
					{message}
				</p>
			)}
			{status === "error" && (
				<p className="rounded-md bg-red-bg p-2.5 text-xs font-semibold text-red">
					{message}
				</p>
			)}
		</form>
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
			<Input
				type="email"
				placeholder="your@email.com"
				value={email}
				onChange={(e) => setEmail(e.target.value)}
				autoComplete="email"
			/>
			<Button
				variant="outline"
				className="w-full text-red"
				disabled={status === "loading"}
				onClick={handleSubmit}
			>
				{status === "loading" ? "Unsubscribing..." : "Unsubscribe"}
			</Button>
			{status === "success" && (
				<p className="rounded-md bg-green-bg p-2.5 text-xs font-semibold text-green">
					{message}
				</p>
			)}
			{status === "error" && (
				<p className="rounded-md bg-red-bg p-2.5 text-xs font-semibold text-red">
					{message}
				</p>
			)}
		</div>
	);
}
