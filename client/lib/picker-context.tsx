"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useState } from "react";

import type { ModelPicker } from "@/types/trade";

const STORAGE_KEY = "vt_model_picker_v1";

interface PickerContextValue {
  picker: ModelPicker;
  setPicker: (next: ModelPicker) => void;
}

const PickerContext = createContext<PickerContextValue | null>(null);

/**
 * Wraps the app and holds the global model filter state. The choice
 * persists in localStorage so it survives page reloads, and every page
 * that fetches trades reads from this context (not from local state)
 * so the filter applies everywhere at once.
 */
export function PickerProvider({ children }: { children: ReactNode }) {
  const [picker, setPickerState] = useState<ModelPicker>("all");

  // Restore on mount.
  useEffect(() => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw === "openai" || raw === "claude" || raw === "all") {
        setPickerState(raw);
      }
    } catch {}
  }, []);

  const setPicker = useCallback((next: ModelPicker) => {
    setPickerState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {}
  }, []);

  return <PickerContext.Provider value={{ picker, setPicker }}>{children}</PickerContext.Provider>;
}

/**
 * Read the current global picker. Falls back to "all" outside the
 * provider so server-side prerender or stand-alone components don't
 * blow up.
 */
export function usePicker(): PickerContextValue {
  const ctx = useContext(PickerContext);
  if (!ctx) {
    return { picker: "all", setPicker: () => {} };
  }
  return ctx;
}
