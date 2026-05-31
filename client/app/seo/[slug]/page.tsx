import { notFound } from "next/navigation";

import { ClaudeLogo } from "@/components/ui/brand-icons";

/*
SEO card pages. Each renders a fixed 1200x630 composition that gets
screenshotted (Chrome headless, 2x) into public/og/<slug>.png and wired
as the og:image / twitter:image for the matching route. Styling is
self-contained and theme-independent (explicit dark-indigo canvas) so a
screenshot looks identical regardless of the visitor's light/dark
preference. These routes are noindex (see robots in metadata) — they
exist only to be captured, not browsed.
*/

export const metadata = { robots: { index: false, follow: false } };

type Card = { eyebrow: string; title: React.ReactNode; subtitle: string };

const CARDS: Record<string, Card> = {
  landing: {
    eyebrow: "Live now · free daily picks at the open",
    title: (
      <>
        One silly model.
        <br />
        <span className="text-gradient-brand">Zero humans.</span>
      </>
    ),
    subtitle: "An LLM picks 3 options contracts before the bell and auto-fires all 3 in a real brokerage account at the open. By close, you see whether it was right.",
  },
  dashboard: {
    eyebrow: "Live dashboard",
    title: (
      <>
        Three real trades,
        <br />
        <span className="text-gradient-brand">ticking live.</span>
      </>
    ),
    subtitle: "Today's auto-fired options picks with marks streaming in real time, each badged as a real Schwab position.",
  },
  history: {
    eyebrow: "Historical analytics",
    title: (
      <>
        Every pick.
        <br />
        <span className="text-gradient-brand">Every outcome.</span>
      </>
    ),
    subtitle: "Win rate, P&L, and conviction calibration across every trade the model has ever fired. Fully public, nothing hidden.",
  },
  faq: {
    eyebrow: "FAQ",
    title: (
      <>
        How the experiment
        <br />
        <span className="text-gradient-brand">actually works.</span>
      </>
    ),
    subtitle: "One model, no humans, real money. The two-call pipeline, the data sources, the guardrails, and the honest odds.",
  },
  terms: {
    eyebrow: "Terms of service",
    title: (
      <>
        Not financial advice.
        <br />
        <span className="text-gradient-brand">Real risk.</span>
      </>
    ),
    subtitle: "An experimental, educational project. Machine-generated suggestions, auto-executed with real money, reviewed by no one.",
  },
  privacy: {
    eyebrow: "Privacy policy",
    title: (
      <>
        Minimal data.
        <br />
        <span className="text-gradient-brand">No trackers.</span>
      </>
    ),
    subtitle: "No analytics SDK, no ad network, no data broker. Just the email you signed up with and a session cookie.",
  },
};

export function generateStaticParams() {
  return Object.keys(CARDS).map((slug) => ({ slug }));
}

export default async function SeoCardPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const card = CARDS[slug];
  if (!card) notFound();

  return (
    <div
      style={{ width: 1200, height: 630, background: "linear-gradient(135deg, #0b0a14 0%, #14122b 55%, #0b0a14 100%)" }}
      className="relative overflow-hidden font-sans text-white"
    >
      {/* Ambient indigo/violet glass orbs */}
      <div className="lg-orb lg-orb-cyan absolute -top-40 -left-32 h-[560px] w-[560px] opacity-50" aria-hidden />
      <div className="lg-orb lg-orb-claude absolute -right-40 -bottom-48 h-[560px] w-[560px] opacity-45" aria-hidden />

      <div className="relative z-10 flex h-full flex-col justify-between px-20 py-16">
        {/* Top: wordmark */}
        <div className="flex items-center justify-between">
          <span className="text-[34px] font-extrabold tracking-tight">
            Vibe<span className="text-gradient-brand">Tradez</span>
          </span>
          <span className="rounded-full border border-white/15 bg-white/[0.06] px-4 py-1.5 text-[15px] font-medium text-white/80">{card.eyebrow}</span>
        </div>

        {/* Middle: headline + subtitle */}
        <div className="max-w-[900px]">
          <h1 className="text-[78px] font-extrabold leading-[1.02] tracking-[-0.03em]">{card.title}</h1>
          <p className="mt-7 max-w-[760px] text-[26px] leading-snug text-white/65">{card.subtitle}</p>
        </div>

        {/* Bottom: powered by */}
        <div className="flex items-center gap-3 text-[19px] text-white/70">
          <span className="text-[14px] uppercase tracking-[0.18em] text-white/45">Powered by</span>
          <ClaudeLogo className="h-6 w-6" />
          <span className="font-semibold text-white/90">Claudia</span>
          <span className="ml-auto text-[18px] text-white/45">vibetradez.com</span>
        </div>
      </div>
    </div>
  );
}
