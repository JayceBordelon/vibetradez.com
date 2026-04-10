import type { Metadata } from "next";
import Link from "next/link";
import {
	ArrowRight,
	BarChart3,
	Brain,
	Mail,
	TrendingUp,
	Zap,
	Shield,
	Clock,
} from "lucide-react";
import { OpenAILogo, ClaudeLogo } from "@/components/ui/brand-icons";

export const metadata: Metadata = {
	title: "VibeTradez | AI-Powered Options Picks",
	description:
		"Free daily ranked options picks powered by two independent AI models. OpenAI GPT-5.4 and Claude Opus 4.6 each analyze sentiment and market data, then deliver ranked trade ideas before market open.",
};

export default function LandingPage() {
	return (
		<div className="min-h-dvh bg-background text-foreground">
			{/* ── Nav ── */}
			<nav className="fixed top-0 z-50 w-full border-b border-border/50 bg-background/80 backdrop-blur-xl">
				<div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
					<span className="text-xl font-extrabold tracking-tight">
						<span className="text-foreground">Vibe</span>
						<span className="text-primary">Tradez</span>
					</span>
					<div className="flex items-center gap-3">
						<Link
							href="/dashboard"
							className="hidden text-sm font-medium text-muted-foreground transition-colors hover:text-foreground sm:block"
						>
							Dashboard
						</Link>
						<Link
							href="/dashboard"
							className="inline-flex items-center gap-2 rounded-lg bg-foreground px-3.5 py-2 text-sm font-semibold text-background transition-opacity hover:opacity-90 sm:px-4"
						>
							<span className="sm:hidden">Dashboard</span>
							<span className="hidden sm:inline">Launch App</span>
							<ArrowRight className="h-4 w-4" />
						</Link>
					</div>
				</div>
			</nav>

			{/* ── Hero ── */}
			<section className="relative flex min-h-dvh items-center justify-center overflow-hidden pt-16">
				{/* Gradient orbs */}
				<div className="pointer-events-none absolute inset-0 overflow-hidden">
					<div className="absolute -top-1/4 left-1/4 h-[600px] w-[600px] rounded-full bg-[#10a37f]/10 blur-[120px] dark:bg-[#10a37f]/5" />
					<div className="absolute -bottom-1/4 right-1/4 h-[600px] w-[600px] rounded-full bg-[#D97757]/10 blur-[120px] dark:bg-[#D97757]/5" />
					<div className="absolute top-1/2 left-1/2 h-[400px] w-[400px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary/5 blur-[100px]" />
				</div>

				{/* Grid pattern */}
				<div
					className="pointer-events-none absolute inset-0 opacity-[0.03] dark:opacity-[0.04]"
					style={{
						backgroundImage:
							"linear-gradient(var(--foreground) 1px, transparent 1px), linear-gradient(90deg, var(--foreground) 1px, transparent 1px)",
						backgroundSize: "60px 60px",
					}}
				/>

				<div className="relative z-10 mx-auto max-w-4xl px-6 text-center">
					{/* Badge */}
					<div className="mb-6 inline-flex items-center gap-2 rounded-full border border-border bg-card/80 px-3 py-1.5 text-xs font-medium text-muted-foreground backdrop-blur-sm sm:mb-8 sm:px-4 sm:py-2 sm:text-sm">
						<span className="relative flex h-2 w-2">
							<span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green opacity-75" />
							<span className="relative inline-flex h-2 w-2 rounded-full bg-green" />
						</span>
						Free daily picks before market open
					</div>

					{/* Headline */}
					<h1 className="mb-5 text-4xl font-extrabold leading-[1.1] tracking-tight sm:mb-6 sm:text-6xl lg:text-7xl">
						Two AIs.
						<br />
						<span className="bg-gradient-to-r from-[#10a37f] to-[#D97757] bg-clip-text text-transparent">
							One Trade List.
						</span>
					</h1>

					<p className="mx-auto mb-8 max-w-2xl text-base leading-relaxed text-muted-foreground sm:mb-10 sm:text-xl">
						Every morning, GPT-5.4 and Claude Opus 4.6 independently
						analyze market sentiment, scan options chains, and each
						produce their top 10 picks. You get the union&mdash;ranked,
						scored, and delivered to your inbox.
					</p>

					{/* CTAs */}
					<div className="flex w-full flex-col gap-3 px-4 sm:w-auto sm:flex-row sm:justify-center sm:gap-4 sm:px-0">
						<Link
							href="/dashboard"
							className="inline-flex items-center justify-center gap-2 rounded-xl bg-foreground px-8 py-3.5 text-base font-semibold text-background shadow-lg transition-all hover:opacity-90 hover:shadow-xl"
						>
							View Live Dashboard
							<ArrowRight className="h-4 w-4" />
						</Link>
						<a
							href="#how-it-works"
							className="inline-flex items-center justify-center gap-2 rounded-xl border border-border px-8 py-3.5 text-base font-semibold text-foreground transition-colors hover:bg-muted"
						>
							See How It Works
						</a>
					</div>

					{/* Model badges */}
					<div className="mt-10 flex items-center justify-center gap-6 sm:mt-12 sm:gap-8">
						<div className="flex items-center gap-2.5 text-sm text-muted-foreground">
							<OpenAILogo className="h-5 w-5" />
							<span className="font-medium">GPT-5.4</span>
						</div>
						<div className="h-4 w-px bg-border" />
						<div className="flex items-center gap-2.5 text-sm text-muted-foreground">
							<ClaudeLogo className="h-5 w-5" />
							<span className="font-medium">Claude Opus 4.6</span>
						</div>
					</div>
				</div>
			</section>

			{/* ── Features ── */}
			<section className="border-t bg-card py-16 sm:py-24">
				<div className="mx-auto max-w-6xl px-5 sm:px-6">
					<div className="mb-10 text-center sm:mb-16">
						<h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
							Built Different
						</h2>
						<p className="mx-auto max-w-2xl text-muted-foreground">
							Not another signal bot. Two frontier AI models run the
							same pipeline independently, then their picks are merged
							and ranked by combined conviction.
						</p>
					</div>

					<div className="grid gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-3">
						{features.map((f) => (
							<div
								key={f.title}
								className="group rounded-2xl border border-border bg-background p-5 transition-all hover:border-primary/30 hover:shadow-md sm:p-6"
							>
								<div className="mb-4 inline-flex rounded-xl bg-muted p-3">
									{f.icon}
								</div>
								<h3 className="mb-2 text-lg font-bold">
									{f.title}
								</h3>
								<p className="text-sm leading-relaxed text-muted-foreground">
									{f.description}
								</p>
							</div>
						))}
					</div>
				</div>
			</section>

			{/* ── How It Works ── */}
			<section id="how-it-works" className="scroll-mt-16 border-t py-16 sm:py-24">
				<div className="mx-auto max-w-5xl px-5 sm:px-6">
					<div className="mb-10 text-center sm:mb-16">
						<h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
							How It Works
						</h2>
						<p className="mx-auto max-w-2xl text-muted-foreground">
							From raw sentiment to ranked picks in your inbox, every
							market morning.
						</p>
					</div>

					<div className="relative">
						{/* Vertical line */}
						<div className="absolute left-6 top-0 bottom-0 hidden w-px bg-border sm:block" />

						<div className="space-y-8 sm:space-y-12">
							{steps.map((step, i) => (
								<div
									key={step.title}
									className="relative flex gap-6"
								>
									<div className="relative z-10 hidden flex-shrink-0 sm:block">
										<div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-border bg-card text-sm font-bold text-primary">
											{i + 1}
										</div>
									</div>
									<div className="flex-1 rounded-2xl border border-border bg-card p-6">
										<div className="mb-1 flex items-center gap-3">
											<span className="inline-flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs font-bold text-primary sm:hidden">
												{i + 1}
											</span>
											<span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
												{step.time}
											</span>
										</div>
										<h3 className="mb-2 text-lg font-bold">
											{step.title}
										</h3>
										<p className="text-sm leading-relaxed text-muted-foreground">
											{step.description}
										</p>
									</div>
								</div>
							))}
						</div>
					</div>
				</div>
			</section>

			{/* ── Dual Model Breakdown ── */}
			<section className="border-t bg-card py-16 sm:py-24">
				<div className="mx-auto max-w-5xl px-5 sm:px-6">
					<div className="mb-10 text-center sm:mb-16">
						<h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
							Dual-Model Conviction
						</h2>
						<p className="mx-auto max-w-2xl text-muted-foreground">
							Each model scores every trade 1&ndash;10 with a written
							rationale. The combined score determines final rank.
						</p>
					</div>

					<div className="grid gap-4 sm:gap-6 md:grid-cols-2">
						{/* OpenAI card */}
						<div className="rounded-2xl border border-[#10a37f]/20 bg-background p-6 sm:p-8">
							<div className="mb-6 flex items-center gap-3">
								<OpenAILogo className="h-8 w-8" />
								<div>
									<h3 className="text-lg font-bold">
										OpenAI GPT-5.4
									</h3>
									<p className="text-sm text-muted-foreground">
										Primary Analyst
									</p>
								</div>
							</div>
							<ul className="space-y-3 text-sm text-muted-foreground">
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#10a37f]" />
									Generates 10 ranked trade ideas from sentiment + live market data
								</li>
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#10a37f]" />
									Multi-turn tool use with Schwab quotes and option chains
								</li>
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#10a37f]" />
									Web search for real-time catalysts and news
								</li>
							</ul>
						</div>

						{/* Claude card */}
						<div className="rounded-2xl border border-[#D97757]/20 bg-background p-6 sm:p-8">
							<div className="mb-6 flex items-center gap-3">
								<ClaudeLogo className="h-8 w-8" />
								<div>
									<h3 className="text-lg font-bold">
										Claude Opus 4.6
									</h3>
									<p className="text-sm text-muted-foreground">
										Independent Validator
									</p>
								</div>
							</div>
							<ul className="space-y-3 text-sm text-muted-foreground">
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#D97757]" />
									Independently picks its own top 10 from the same raw data
								</li>
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#D97757]" />
									Same Schwab + web search tool access as GPT
								</li>
								<li className="flex items-start gap-2">
									<Zap className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#D97757]" />
									Flags concerns and red flags GPT may have missed
								</li>
							</ul>
						</div>
					</div>

					<div className="mt-8 rounded-2xl border border-border bg-background p-6 text-center">
						<p className="text-sm font-semibold text-foreground">
							Combined Score = (GPT Score + Claude Score) / 2
						</p>
						<p className="mt-1 text-xs text-muted-foreground">
							Trades are re-ranked by combined conviction. Claude
							breaks ties. Both rationales are visible on the
							dashboard.
						</p>
					</div>
				</div>
			</section>

			{/* ── CTA ── */}
			<section className="border-t py-16 sm:py-24">
				<div className="mx-auto max-w-3xl px-5 text-center sm:px-6">
					<h2 className="mb-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
						Start Getting Picks
					</h2>
					<p className="mb-8 text-muted-foreground">
						Completely free. No credit card. Unsubscribe any time.
						Picks arrive before every market open with end-of-day
						results tracked automatically.
					</p>
					<div className="flex w-full flex-col gap-3 px-4 sm:w-auto sm:flex-row sm:justify-center sm:gap-4 sm:px-0">
						<Link
							href="/dashboard"
							className="inline-flex items-center justify-center gap-2 rounded-xl bg-foreground px-8 py-3.5 text-base font-semibold text-background shadow-lg transition-all hover:opacity-90"
						>
							Open Dashboard
							<ArrowRight className="h-4 w-4" />
						</Link>
						<Link
							href="/faq"
							className="inline-flex items-center justify-center gap-2 rounded-xl border border-border px-8 py-3.5 text-base font-semibold text-foreground transition-colors hover:bg-muted"
						>
							Read the FAQ
						</Link>
					</div>
				</div>
			</section>

			{/* ── Footer ── */}
			<footer className="border-t bg-card">
				<div className="mx-auto flex max-w-6xl flex-col gap-4 px-6 py-8 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
					<div className="flex items-center gap-2">
						<span className="font-extrabold text-foreground">
							Vibe<span className="text-primary">Tradez</span>
						</span>
						<span>&copy; {new Date().getFullYear()}</span>
					</div>
					<p className="max-w-lg leading-relaxed">
						Not financial advice. Options trading involves substantial
						risk. All P&amp;L figures are hypothetical. Past
						performance does not guarantee future results.
					</p>
					<div className="flex gap-4">
						<Link
							href="/terms"
							className="underline underline-offset-2 hover:text-foreground"
						>
							Terms
						</Link>
						<Link
							href="/faq"
							className="underline underline-offset-2 hover:text-foreground"
						>
							FAQ
						</Link>
						<a
							href="https://jaycebordelon.com"
							target="_blank"
							rel="noopener noreferrer"
							className="underline underline-offset-2 hover:text-foreground"
						>
							Built by Jayce
						</a>
					</div>
				</div>
			</footer>
		</div>
	);
}

const features = [
	{
		icon: <Brain className="h-6 w-6 text-primary" />,
		title: "Dual-Model Analysis",
		description:
			"Two frontier AI models analyze independently. No groupthink. GPT-5.4 picks, Claude Opus 4.6 picks, and you get the combined ranking.",
	},
	{
		icon: <TrendingUp className="h-6 w-6 text-primary" />,
		title: "Live Market Data",
		description:
			"Real-time quotes and full option chains from Schwab's API. Both models use multi-turn tool calling to research before they pick.",
	},
	{
		icon: <BarChart3 className="h-6 w-6 text-primary" />,
		title: "Full Transparency",
		description:
			"Every trade shows both models' scores, rationales, and any red flags. Historical performance, equity curves, and model comparison all tracked.",
	},
	{
		icon: <Mail className="h-6 w-6 text-primary" />,
		title: "Pre-Market Email",
		description:
			"Ranked picks in your inbox before 9:30 AM ET every trading day. End-of-day results follow at market close. Weekly digest on Fridays.",
	},
	{
		icon: <Shield className="h-6 w-6 text-primary" />,
		title: "Completely Free",
		description:
			"No paywalls, no premium tiers, no credit card. This is a live experiment in dual-model trading, open for anyone to follow along.",
	},
	{
		icon: <Clock className="h-6 w-6 text-primary" />,
		title: "End-of-Day Tracking",
		description:
			"Every pick is tracked to close. Win rates, P&L, Sharpe ratio, max drawdown, and profit factor are computed automatically. No cherry-picking.",
	},
];

const steps = [
	{
		time: "9:00 AM ET",
		title: "Sentiment Scan",
		description:
			"The system scrapes Reddit (r/wallstreetbets, r/options) for trending tickers and sentiment signals. This raw data feeds both AI models.",
	},
	{
		time: "9:15 AM ET",
		title: "Dual-Model Analysis",
		description:
			"GPT-5.4 and Claude Opus 4.6 each receive the sentiment data and independently call Schwab for live quotes and option chains. Each produces 10 ranked picks with conviction scores and written rationales.",
	},
	{
		time: "9:25 AM ET",
		title: "Merge, Rank & Deliver",
		description:
			"Picks from both models are unioned, scored by combined conviction (average of both scores), and re-ranked. The final list is emailed to all subscribers before the opening bell.",
	},
	{
		time: "4:05 PM ET",
		title: "End-of-Day Results",
		description:
			"After close, the system fetches closing prices, computes hypothetical P&L for every pick, and emails results. Everything is saved to the database and surfaces on the dashboard.",
	},
];
