"use client";

import { ClaudeLogo, OpenAILogo } from "@/components/ui/brand-icons";
import { usePicker } from "@/lib/picker-context";
import { cn } from "@/lib/utils";
import type { ModelPicker } from "@/types/trade";

const OPTIONS: { value: ModelPicker; label: string; Icon?: typeof OpenAILogo }[] = [
  { value: "all", label: "All" },
  { value: "openai", label: "OpenAI", Icon: OpenAILogo },
  { value: "claude", label: "Claude", Icon: ClaudeLogo },
];

/**
 * Compact segmented control rendered inside the NavBar. Reads from
 * and writes to the global PickerContext, so toggling here updates
 * every dashboard / history fetch instantly across the app.
 */
export function ModelPickerFilter() {
  const { picker, setPicker } = usePicker();

  return (
    <div role="radiogroup" aria-label="Model filter" className="flex shrink-0 items-stretch gap-1 self-center rounded-md border bg-card p-1">
      {OPTIONS.map((opt) => {
        const active = picker === opt.value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => setPicker(opt.value)}
            className={cn(
              "flex items-center gap-1.5 rounded px-2.5 py-1 text-xs font-semibold transition-colors",
              active && opt.value === "openai"
                ? "bg-gpt-light text-gpt"
                : active && opt.value === "claude"
                  ? "bg-claude-light text-claude"
                  : active
                    ? "bg-gradient-brand text-white"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
          >
            {opt.Icon && <opt.Icon className="h-3.5 w-3.5" />}
            <span className={cn(opt.Icon && "hidden sm:inline")}>{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
