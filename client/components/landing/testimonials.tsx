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
  /** Tailwind classes for the avatar gradient, keep them brand-tinted
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
    avatarClass: "bg-gradient-to-br from-sky-400 to-cyan-500 text-white",
  },
  {
    name: "Elon Musk",
    title: "Chief Vibe Officer, X/AI/SpaceX/Boring/Tesla/Neuralink",
    initials: "EM",
    rating: 3,
    quote: "I bought 420 calls expiring 4/20 because Claudia told me to. Should've read the disclaimer. Would have been five stars but my Cybertruck ran out of options premium.",
    avatarClass: "bg-gradient-to-br from-rose-400 to-orange-500 text-white",
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
    avatarClass: "bg-gradient-to-br from-emerald-400 to-teal-500 text-white",
  },
  {
    name: "Bill Gates",
    title: "Co-founder of a Phone OS You Forgot About",
    initials: "BG",
    rating: 4.5,
    quote: "I asked Copilot to subscribe. It bought seventeen contracts on a ticker that doesn't exist. Bullish. Half star off because Clippy didn't get a kickback.",
    avatarClass: "bg-gradient-to-br from-cyan-400 to-blue-500 text-white",
  },
  {
    name: "Donald Trump",
    title: "Former, Future, Current President (Depending)",
    initials: "DT",
    rating: 5,
    quote: "TREMENDOUS picks. The best options. Made four trillion dollars in my mind alone. Sad I didn't think of it first. Five out of four stars.",
    avatarClass: "bg-gradient-to-br from-red-400 to-amber-500 text-white",
  },
  {
    name: "Taylor Swift",
    title: "Billionaire by Sheer Force of Friendship Bracelets",
    initials: "TS",
    rating: 5,
    quote: "Read Claudia's rationale and the third letter of each sentence spelled 'TS'. Dropping a re-recorded album about a covered call gone wrong. Stream it.",
    avatarClass: "bg-gradient-to-br from-pink-400 to-rose-500 text-white",
  },
  {
    name: "Tim Cook",
    title: "CEO, Apple (Allegedly Still Sells iPhones)",
    initials: "TC",
    rating: 4,
    quote: "VibeTradez integrates beautifully with our ecosystem. We are patenting the swipe-down gesture for rejecting Claudia's worst picks. Coming to iOS 19, probably.",
    avatarClass: "bg-gradient-to-br from-slate-400 to-zinc-500 text-white",
  },
  {
    name: "Jensen Huang",
    title: "Founder, NVIDIA (Owns Three Datacenters Personally)",
    initials: "JH",
    rating: 5,
    quote: "Bought 9 calls. They were going to be H100s but the supply chain is constrained. Claudia says it doesn't matter. Leather jacket noted.",
    avatarClass: "bg-gradient-to-br from-lime-400 to-green-500 text-white",
  },
  {
    name: "Larry Ellison",
    title: "Chairman, Oracle (Currently on the Yacht)",
    initials: "LE",
    rating: 4.5,
    quote: "Parked the yacht at the dock and asked Claudia what to buy. It said Oracle. I am reporting myself to myself for insider tips.",
    avatarClass: "bg-gradient-to-br from-blue-400 to-sky-500 text-white",
  },
  {
    name: "Satya Nadella",
    title: "CEO, Microsoft (Empathetic Vibes Quarter)",
    initials: "SN",
    rating: 4,
    quote: "Copilot tried to subscribe but the form refused to autofill from Edge. We have added 'autofill' to our Q4 OKRs. Bullish on synergies.",
    avatarClass: "bg-gradient-to-br from-cyan-400 to-blue-500 text-white",
  },
  {
    name: "Mr Beast",
    title: "CEO, Beast Burger / Feastables / Beast Philanthropy",
    initials: "MB",
    rating: 5,
    quote: "Gave 100 strangers $10,000 each and told them to follow VibeTradez. 99 went broke. The 100th bought MAGS calls. Subscribe to my channel.",
    avatarClass: "bg-gradient-to-br from-orange-400 to-red-500 text-white",
  },
  {
    name: "Snoop Dogg",
    title: "Doggfather (Diversified Holdings)",
    initials: "SD",
    rating: 4.5,
    quote: "Claudia said sell. I told her to drop a chart. The chart was bullish. We are now business partners in a vape company.",
    avatarClass: "bg-gradient-to-br from-teal-400 to-emerald-500 text-white",
  },
  {
    name: "Marc Andreessen",
    title: "GP, Andreessen Horowitz (Tweeting Through It)",
    initials: "MA",
    rating: 5,
    quote: "Software is eating the world. Claudia is eating software. I am eating optimism. We are so back. (a16z is also so back.)",
    avatarClass: "bg-gradient-to-br from-zinc-400 to-neutral-500 text-white",
  },
  {
    name: "Vitalik Buterin",
    title: "Co-founder, Ethereum (Currently in Argentina)",
    initials: "VB",
    rating: 5,
    quote: "Claudia proposed an EIP for option premiums on Layer 2. Vibrant intellectual energy. The merge will resolve all losing positions retroactively.",
    avatarClass: "bg-gradient-to-br from-lime-400 to-green-500 text-white",
  },
];

/*
Round-robin testimonials into 3 columns so adding new entries keeps
distribution balanced automatically. Each column then animates at a
different speed for a parallax wall effect. On mobile the wall
collapses to a single horizontal auto-scrolling marquee (see
Testimonials) since a vertical auto-scroll wall inside a
vertically-scrolling page is hostile UX and a full static stack scrolls
forever.
*/
const COLUMN_GROUPS: Testimonial[][] = (() => {
  const cols: Testimonial[][] = [[], [], []];
  TESTIMONIALS.forEach((t, i) => {
    cols[i % 3].push(t);
  });
  return cols;
})();
const COLUMN_DURATIONS = ["68s", "54s", "78s"];

export function Testimonials() {
  return (
    <section className="relative px-5 pb-28 sm:px-6 sm:pb-40">
      <div className="relative z-10 mx-auto max-w-6xl">
        <Reveal effect="blur" duration={1000} className="mx-auto mb-10 max-w-2xl text-center sm:mb-14">
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Five-star reviews</span>
          <h2 className="mt-3 text-3xl font-extrabold tracking-tight sm:text-4xl">
            From <span className="text-gradient-brand">definitely real</span> people
          </h2>
          <p className="mt-3 text-sm text-muted-foreground sm:text-base">Loved by titans of industry. Allegedly.</p>
        </Reveal>

        {/* Mobile: auto-scrolling horizontal marquee, mirrors the desktop
            wall. Doubled track + .animate-marquee (translateX 0 -> -50%)
            loops seamlessly; a 16-card vertical stack scrolled forever and
            a manual swipe needed a nudge, so it auto-advances instead.
            CSS-only, no JS; respects prefers-reduced-motion via globals. */}
        <div className="-mx-5 overflow-hidden sm:hidden [-webkit-mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)] [mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)]">
          <div className="flex w-max animate-marquee gap-4 py-1" style={{ animationDuration: "85s" }}>
            {[...TESTIMONIALS, ...TESTIMONIALS].map((t, i) => (
              <div key={`${t.name}-${i}`} aria-hidden={i >= TESTIMONIALS.length || undefined} className="flex w-[78vw] max-w-[340px] shrink-0">
                <TestimonialCard t={t} />
              </div>
            ))}
          </div>
        </div>

        {/* Desktop: scrolling parallax columns */}
        <div className="relative hidden h-[640px] gap-5 overflow-hidden sm:grid sm:grid-cols-2 lg:grid-cols-3 [mask-image:linear-gradient(to_bottom,transparent,black_10%,black_90%,transparent)] [-webkit-mask-image:linear-gradient(to_bottom,transparent,black_10%,black_90%,transparent)]">
          {COLUMN_GROUPS.map((col, idx) => (
            <div key={`col-${idx}`} className={cn("flex animate-marquee-up flex-col gap-5", idx === 2 && "hidden lg:flex")} style={{ animationDuration: COLUMN_DURATIONS[idx] }}>
              {[...col, ...col].map((t, i) => (
                <TestimonialCard key={`${t.name}-${i}`} t={t} hidden={i >= col.length} />
              ))}
            </div>
          ))}
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

function TestimonialCard({ t, hidden }: { t: Testimonial; hidden?: boolean }) {
  return (
    <article
      aria-hidden={hidden || undefined}
      className="group flex w-full shrink-0 flex-col gap-4 rounded-2xl border border-foreground/[0.08] bg-foreground/[0.015] p-6 transition-all duration-300 hover:border-foreground/20 hover:bg-foreground/[0.04] dark:border-white/[0.06] dark:bg-white/[0.015] dark:hover:border-white/15 dark:hover:bg-white/[0.04]"
    >
      <div className="flex items-center justify-between gap-3">
        <Stars rating={t.rating} />
        <Quote className="h-4 w-4 shrink-0 text-foreground/20" aria-hidden />
      </div>
      <p className="text-[15px] leading-relaxed text-foreground/85">&ldquo;{t.quote}&rdquo;</p>
      <div className="mt-auto flex items-start gap-3 border-t border-foreground/5 pt-4 dark:border-white/5">
        <div className={cn("flex h-9 w-9 shrink-0 items-center justify-center rounded-full font-mono text-[11px] font-bold", t.avatarClass)} aria-hidden>
          {t.initials}
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold tracking-tight">{t.name}</div>
          <div className="line-clamp-2 text-[11px] leading-snug text-muted-foreground">{t.title}</div>
        </div>
      </div>
    </article>
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
