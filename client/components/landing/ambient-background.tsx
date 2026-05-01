/*
Ambient mesh canvas. Renders a single continuous mesh-gradient wash
behind every page so the background reads as a soft brand-tinted
field instead of a set of discrete colored "orbs" that draw the
eye to specific spots. The mesh layer transforms as one element
(slow scale + drift) so the wash subtly breathes without any
moving parts the eye can lock onto.

Use `fixed` for inner pages where the page is taller than the
viewport, `absolute` for self-contained sections (the landing hero).
The default is `fixed` since most callers want full-page coverage.

Density:
  full   Full-strength mesh. Used on the landing where the
         background is a feature.
  muted  Mesh + wrapper opacity-65 so the wash recedes behind
         data-heavy inner pages but still tints the surface.
*/
interface AmbientBackgroundProps {
  position?: "fixed" | "absolute";
  density?: "full" | "muted";
}

export function AmbientBackground({ position = "fixed", density = "full" }: AmbientBackgroundProps) {
  const isMuted = density === "muted";
  const wrapperOpacity = isMuted ? "opacity-65" : "";

  return (
    <div className={`pointer-events-none ${position === "fixed" ? "fixed" : "absolute"} inset-0 z-0 overflow-hidden ${wrapperOpacity}`} aria-hidden>
      <div className="lg-mesh" />
      <div className="lg-noise" />
    </div>
  );
}
