/*
Ambient orb canvas. Stacks 5 saturated radial gradients at heavy blur
plus a film-grain overlay so panels rendered above pick up colored
light bleed and a tactile finish. Uses fixed positioning so it stays
parked behind every viewport scroll on inner pages, and absolute
positioning on the landing where it's nested inside the hero stack.

Pass `fixed` for inner pages where the page is taller than the
viewport, `absolute` for self-contained sections (the landing hero).
The default is `fixed` since most callers want full-page coverage.
*/
interface AmbientBackgroundProps {
  position?: "fixed" | "absolute";
  density?: "full" | "muted";
}

export function AmbientBackground({ position = "fixed", density = "full" }: AmbientBackgroundProps) {
  const opacityMod = density === "muted" ? "opacity-70" : "";
  return (
    <div className={`pointer-events-none ${position === "fixed" ? "fixed" : "absolute"} inset-0 z-0 overflow-hidden ${opacityMod}`} aria-hidden>
      <div className="lg-orb lg-orb-claude h-[720px] w-[720px] -top-40 -left-32 sm:-top-60 sm:-left-40" />
      <div className="lg-orb lg-orb-amber h-[620px] w-[620px] top-[18%] right-[-15%] sm:top-[10%]" />
      <div className="lg-orb lg-orb-rose h-[540px] w-[540px] top-[55%] left-[-10%]" />
      <div className="lg-orb lg-orb-cyan h-[560px] w-[560px] top-[70%] right-[-12%]" />
      <div className="lg-orb lg-orb-violet h-[480px] w-[480px] bottom-[-10%] left-[35%]" />
      <div className="lg-noise" />
    </div>
  );
}
