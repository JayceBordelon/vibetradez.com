import { Quote, Star } from "lucide-react";

import { cn } from "@/lib/utils";

import { Reveal } from "./reveal";

/*
Clearly-satirical testimonials section. Every "person" here is a famous
public figure whose quote is obviously fabricated; the section header,
disclaimer, and tone make sure no reader could mistake this for a real
endorsement. None of these people have any connection to VibeTradez.
*/
interface Testimonial {
  name: string;
  title: string;
  initials: string;
  rating: number;
  quote: string;
  /** Tailwind classes for the avatar gradient — keep them brand-tinted
   *  but distinct so the row of avatars reads as varied at a glance. */
  avatarClass: string;
}

const TESTIMONIALS: Testimonial[] = [
  {
    name: "Jeff Bezos",
    title: "Recently Demoted Billionaire",
    initials: "JB",
    rating: 4.5,
    quote: "I didn't know much about trading so I gave all my money to VibeTradez. Now I'm the poorest man alive. Half a star off because the email subject line uses an em-dash.",
    avatarClass: "bg-gradient-to-br from-amber-400 to-orange-500 text-white",
  },
  {
    name: "Mark Zuckerberg",
    title: "CEO, A Reasonable Lizard",
    initials: "MZ",
    rating: 5,
    quote: "VibeTradez taught me that money is just the metaverse for stocks. My wife took the kids and the chickens. 5/5, would lose everything again.",
    avatarClass: "bg-gradient-to-br from-sky-400 to-indigo-500 text-white",
  },
  {
    name: "Elon Musk",
    title: "Chief Vibe Officer, X/AI/SpaceX/Boring/Tesla/Neuralink",
    initials: "EM",
    rating: 3,
    quote: "I bought 420 calls expiring 4/20 because Claude told me to. Should've read the disclaimer. Would have been five stars but my Cybertruck ran out of options premium.",
    avatarClass: "bg-gradient-to-br from-rose-400 to-fuchsia-500 text-white",
  },
  {
    name: "Warren Buffett",
    title: "Oracle of Omaha (Retired, Reluctant)",
    initials: "WB",
    rating: 4,
    quote: "I asked Charlie what he thought of VibeTradez. He's been gone two years. Answer my texts, Charlie. Four stars for the company in this difficult time.",
    avatarClass: "bg-gradient-to-br from-emerald-400 to-teal-500 text-white",
  },
  {
    name: "Sam Altman",
    title: "Definitely Not A Trader, Just Asking Questions",
    initials: "SA",
    rating: 5,
    quote: "VibeTradez generated seven trillion dollars of imaginary alpha. Imaginary chips. Imaginary fab. Open my IPO already. Five out of five Worldcoins.",
    avatarClass: "bg-gradient-to-br from-violet-400 to-purple-500 text-white",
  },
  {
    name: "Bill Gates",
    title: "Co-founder of a Phone OS You Forgot About",
    initials: "BG",
    rating: 4.5,
    quote: "I asked Copilot to subscribe. It bought seventeen contracts on a ticker that doesn't exist. Bullish. Half star off because Clippy didn't get a kickback.",
    avatarClass: "bg-gradient-to-br from-cyan-400 to-blue-500 text-white",
  },
];

export function Testimonials() {
  return (
    <section className="relative px-5 pb-20 sm:px-6 sm:pb-28">
      <div className="relative z-10 mx-auto max-w-6xl">
        <Reveal effect="blur" duration={1000} className="mx-auto mb-10 max-w-2xl text-center sm:mb-14">
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Five-star reviews</span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
            From <span className="text-gradient-brand">definitely real</span> people
          </h2>
          <p className="mt-3 text-sm text-muted-foreground sm:text-base">Loved by titans of industry. Allegedly.</p>
        </Reveal>

        <div className="grid gap-4 sm:grid-cols-2 sm:gap-5 lg:grid-cols-3">
          {TESTIMONIALS.map((t, i) => {
            const col = i % 3;
            const row = Math.floor(i / 3);
            const stagger = (col + row) * 90;
            return (
              <Reveal key={t.name} effect="rise" delay={stagger} duration={700}>
                <article className="lg-panel lg-edge-shine group flex h-full flex-col gap-4 p-6 transition-transform duration-300 hover:-translate-y-0.5">
                  <div className="flex items-center justify-between gap-3">
                    <Stars rating={t.rating} />
                    <Quote className="h-4 w-4 shrink-0 text-foreground/20" aria-hidden />
                  </div>
                  <p className="flex-1 text-[15px] leading-relaxed text-foreground/85">&ldquo;{t.quote}&rdquo;</p>
                  <div className="flex items-start gap-3 border-t border-foreground/5 pt-4 dark:border-white/5">
                    <div className={cn("flex h-9 w-9 shrink-0 items-center justify-center rounded-full font-mono text-[11px] font-bold", t.avatarClass)} aria-hidden>
                      {t.initials}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-semibold tracking-tight">{t.name}</div>
                      <div className="line-clamp-2 text-[11px] leading-snug text-muted-foreground">{t.title}</div>
                    </div>
                  </div>
                </article>
              </Reveal>
            );
          })}
        </div>

        <Reveal effect="fade" duration={800} delay={300} className="mt-8 text-center">
          <p className="text-[11px] italic text-muted-foreground/70">
            All names, likenesses, and quotes are used in a parody / satire context. No endorsement implied. No billionaires were harmed in the making of this section.
          </p>
        </Reveal>
      </div>
    </section>
  );
}

function Stars({ rating }: { rating: number }) {
  const rounded = Math.round(rating * 2) / 2;
  return (
    <div className="flex items-center gap-0.5" role="img" aria-label={`${rating} out of 5 stars`}>
      {Array.from({ length: 5 }).map((_, i) => {
        const filled = rounded >= i + 1;
        const half = !filled && rounded >= i + 0.5;
        return (
          <span key={`star-${i}`} className="relative inline-block h-3.5 w-3.5">
            <Star className="absolute inset-0 h-3.5 w-3.5 text-amber-400/30" fill="currentColor" strokeWidth={0} />
            {(filled || half) && (
              <span className="absolute inset-0 overflow-hidden" style={{ width: filled ? "100%" : "50%" }}>
                <Star className="h-3.5 w-3.5 text-amber-400" fill="currentColor" strokeWidth={0} />
              </span>
            )}
          </span>
        );
      })}
      <span className="ml-1 text-[11px] font-semibold tabular-nums text-muted-foreground">{rating.toFixed(1)}</span>
    </div>
  );
}
