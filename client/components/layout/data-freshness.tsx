"use client";

import type * as React from "react";

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type FreshnessState = "live" | "market-closed" | "pre-market" | "loading" | "stale";

interface DataFreshnessProps {
  state: FreshnessState;
  asOf?: string;
}

function fmtET(iso?: string): string {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "2-digit",
    timeZone: "America/New_York",
  });
}

interface StateConfig {
  label: string;
  dotColor: string;
  ringColor: string;
  showPing: boolean;
  description: string;
  trailing?: (asOf?: string) => React.ReactNode;
}

const STATE_CONFIG: Record<FreshnessState, StateConfig> = {
  live: {
    label: "LIVE",
    dotColor: "bg-green",
    ringColor: "bg-green",
    showPing: true,
    description: "Streaming live market data.",
    trailing: (asOf) => {
      const t = fmtET(asOf);
      return t ? <small className="text-muted-foreground">{t} ET</small> : null;
    },
  },
  "market-closed": {
    label: "Market closed",
    dotColor: "bg-muted-foreground",
    ringColor: "bg-muted-foreground",
    showPing: false,
    description: "U.S. equities market is closed. Showing the most recent snapshot.",
    trailing: (asOf) => {
      const t = fmtET(asOf);
      return t ? <small className="text-muted-foreground">{t} ET</small> : null;
    },
  },
  "pre-market": {
    label: "Pre-market",
    dotColor: "bg-amber",
    ringColor: "bg-amber",
    showPing: false,
    description: "Pre-market session. Daily picks publish at 9:25 AM ET.",
    trailing: () => <small className="text-muted-foreground">Picks publish at 9:25 AM ET</small>,
  },
  loading: {
    label: "Loading\u2026",
    dotColor: "bg-primary",
    ringColor: "bg-primary",
    showPing: true,
    description: "Fetching the latest data.",
  },
  stale: {
    label: "Data >5 min old",
    dotColor: "bg-red",
    ringColor: "bg-red",
    showPing: false,
    description: "Data hasn\u2019t refreshed in more than 5 minutes. Check your connection.",
    trailing: (asOf) => {
      const t = fmtET(asOf);
      return t ? <small className="text-muted-foreground">{t} ET</small> : null;
    },
  },
};

export function DataFreshness({ state, asOf }: DataFreshnessProps): React.JSX.Element {
  const cfg = STATE_CONFIG[state];
  const trailing = cfg.trailing?.(asOf);

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={cn("lg-pill inline-flex items-center gap-2 px-3 py-1.5 text-xs font-medium")}>
            <span className="relative flex h-2 w-2">
              {cfg.showPing && <span className={cn("absolute inline-flex h-full w-full animate-ping rounded-full opacity-75 [animation-duration:2s]", cfg.ringColor)} />}
              <span className={cn("relative inline-flex h-2 w-2 rounded-full", cfg.dotColor)} />
            </span>
            <span>{cfg.label}</span>
            {trailing}
          </span>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          <div className="flex flex-col gap-0.5">
            <span>{cfg.description}</span>
            {asOf && <span className="text-[11px] opacity-70">{asOf}</span>}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
