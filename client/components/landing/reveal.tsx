"use client";

import { motion, type Variants } from "motion/react";

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

/*
Reveal animates its child into view the first time the element
crosses the viewport's lower threshold. The IntersectionObserver
plumbing is delegated to motion's `whileInView` so we get a single
declarative API and free integration with prefers-reduced-motion
(motion automatically becomes a no-op when the user opts out).

The variant set covers the 8 effects callers used in the previous
hand-rolled implementation, so call-sites in app/page.tsx keep
working unchanged.
*/
const VARIANTS: Record<Effect, Variants> = {
  rise: {
    hidden: { opacity: 0, y: 32, filter: "blur(2px)" },
    show: { opacity: 1, y: 0, filter: "blur(0px)" },
  },
  fall: {
    hidden: { opacity: 0, y: -24 },
    show: { opacity: 1, y: 0 },
  },
  left: {
    hidden: { opacity: 0, x: -40 },
    show: { opacity: 1, x: 0 },
  },
  right: {
    hidden: { opacity: 0, x: 40 },
    show: { opacity: 1, x: 0 },
  },
  scale: {
    hidden: { opacity: 0, scale: 0.9, filter: "blur(3px)" },
    show: { opacity: 1, scale: 1, filter: "blur(0px)" },
  },
  tilt: {
    hidden: { opacity: 0, y: 24, rotate: -2 },
    show: { opacity: 1, y: 0, rotate: 0 },
  },
  blur: {
    hidden: { opacity: 0, scale: 1.04, filter: "blur(6px)" },
    show: { opacity: 1, scale: 1, filter: "blur(0px)" },
  },
  fade: {
    hidden: { opacity: 0 },
    show: { opacity: 1 },
  },
};

const EASE_OUT = [0.16, 1.16, 0.3, 1] as const;
const EASE_FADE = [0.22, 1, 0.36, 1] as const;

export function Reveal({ children, className, delay = 0, duration = 800, effect = "rise", as = "div" }: RevealProps) {
  const Component = motion[as];
  const ease = effect === "fade" || effect === "blur" ? EASE_FADE : EASE_OUT;

  return (
    <Component
      className={cn("will-change-transform", className)}
      variants={VARIANTS[effect]}
      initial="hidden"
      whileInView="show"
      viewport={{ once: true, amount: 0.12, margin: "0px 0px -8% 0px" }}
      transition={{ duration: duration / 1000, delay: delay / 1000, ease }}
    >
      {children}
    </Component>
  );
}
