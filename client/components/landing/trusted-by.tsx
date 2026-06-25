"use client";

import { Anchor, Beef, Bird, Coffee, Crown, Eye, Gamepad2, Landmark, Skull, Sparkles } from "lucide-react";
import type * as React from "react";

interface Brand {
  Icon: React.ComponentType<{ className?: string; "aria-hidden"?: boolean }>;
  name: string;
}

/*
Parody "trusted by" row. Alternates silly americana (Wendy's, Beanie
Babies) with finance super-villains and surveillance giants (Palantir,
Citadel, Goldman, Lehman, Bridgewater). The juxtaposition is the joke.
Icons are Lucide stand-ins gesturing at each brand so the row reads as
one consistent monochrome logo strip. Order is interleaved for max
contrast as it scrolls.
*/
const BRANDS: Brand[] = [
  { Icon: Beef, name: "Wendy's" },
  { Icon: Eye, name: "Palantir" },
  { Icon: Sparkles, name: "Beanie Babies" },
  { Icon: Landmark, name: "Goldman Sachs" },
  { Icon: Bird, name: "Hooters" },
  { Icon: Crown, name: "Citadel" },
  { Icon: Gamepad2, name: "GameStop" },
  { Icon: Coffee, name: "Dunkin'" },
  { Icon: Skull, name: "Lehman Brothers" },
  { Icon: Anchor, name: "Bridgewater" },
];

export function TrustedBy() {
  return (
    <section className="relative px-5 sm:px-6" aria-label="Trusted by, allegedly">
      <div className="relative z-10 mx-auto max-w-6xl">
        <p className="mb-7 text-center text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground sm:mb-9">
          Trusted by <span className="ml-1 normal-case italic tracking-normal text-muted-foreground/70">(allegedly)</span>
        </p>

        <div className="relative overflow-hidden [mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)] [-webkit-mask-image:linear-gradient(to_right,transparent,black_8%,black_92%,transparent)]">
          <div className="flex w-max animate-marquee items-center gap-14 py-1 sm:gap-20">
            {[...BRANDS, ...BRANDS].map((b, i) => (
              <BrandItem key={`${b.name}-${i}`} Icon={b.Icon} name={b.name} hidden={i >= BRANDS.length} />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function BrandItem({ Icon, name, hidden }: Brand & { hidden: boolean }) {
  return (
    <div aria-hidden={hidden || undefined} className="flex shrink-0 items-center gap-2 text-foreground/55 transition-colors hover:text-foreground/90">
      <Icon className="h-5 w-5" aria-hidden />
      <span className="whitespace-nowrap text-base font-semibold tracking-tight sm:text-lg">{name}</span>
    </div>
  );
}
