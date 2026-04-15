"use client";

import type * as React from "react";

interface PageToolbarProps {
  leftControls?: React.ReactNode;
  rightSlot?: React.ReactNode;
}

export function PageToolbar({ leftControls, rightSlot }: PageToolbarProps): React.JSX.Element | null {
  if (!leftControls && !rightSlot) return null;

  return (
    <div className="border-b bg-card/40">
      <div className="mx-auto flex max-w-[1200px] flex-wrap items-center justify-between gap-2 px-4 py-2 sm:gap-3 sm:px-7">
        <div className="flex flex-wrap items-center gap-2 sm:gap-3">{leftControls}</div>
        {rightSlot && <div className="flex shrink-0 items-center">{rightSlot}</div>}
      </div>
    </div>
  );
}
