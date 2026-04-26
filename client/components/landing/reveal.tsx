"use client";

import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

type Effect = "rise" | "fall" | "left" | "right" | "scale" | "tilt" | "blur" | "fade";

interface RevealProps {
  children?: React.ReactNode;
  className?: string;
  delay?: number;
  duration?: number;
  effect?: Effect;
  as?: "div" | "section" | "li" | "article" | "header" | "span";
}

const HIDDEN: Record<Effect, string> = {
  rise: "translate-y-8 opacity-0 blur-[2px]",
  fall: "-translate-y-6 opacity-0",
  left: "-translate-x-10 opacity-0",
  right: "translate-x-10 opacity-0",
  scale: "scale-90 opacity-0 blur-[3px]",
  tilt: "translate-y-6 -rotate-2 opacity-0",
  blur: "scale-[1.04] opacity-0 blur-[6px]",
  fade: "opacity-0",
};

const SHOWN = "translate-x-0 translate-y-0 rotate-0 scale-100 opacity-100 blur-0";

export function Reveal({ children, className, delay = 0, duration = 800, effect = "rise", as = "div" }: RevealProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setVisible(true);
      return;
    }
    /**
    If the element is already in (or above) the viewport at mount, reveal
    synchronously so SEO/OG snapshots and screen-readers see content
    without waiting for an IntersectionObserver tick.
    */
    const r = el.getBoundingClientRect();
    if (r.bottom <= window.innerHeight && r.top >= 0) {
      setVisible(true);
      return;
    }
    if (typeof IntersectionObserver === "undefined") {
      setVisible(true);
      return;
    }
    const obs = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            setVisible(true);
            obs.unobserve(e.target);
          }
        }
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.12 }
    );
    obs.observe(el);
    return () => obs.disconnect();
  }, []);

  const Tag = as as React.ElementType;
  return (
    <Tag
      ref={ref}
      style={{
        transitionDelay: delay ? `${delay}ms` : undefined,
        transitionDuration: `${duration}ms`,
        // Slight overshoot for a springy settle on transform-based effects.
        transitionTimingFunction: effect === "fade" || effect === "blur" ? "cubic-bezier(0.22, 1, 0.36, 1)" : "cubic-bezier(0.16, 1.16, 0.3, 1)",
      }}
      className={cn(
        "transition-[opacity,transform,filter] will-change-transform motion-reduce:transition-none motion-reduce:transform-none motion-reduce:blur-0",
        visible ? SHOWN : HIDDEN[effect],
        className
      )}
    >
      {children}
    </Tag>
  );
}
