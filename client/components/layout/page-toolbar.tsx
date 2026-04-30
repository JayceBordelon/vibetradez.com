"use client";

import type * as React from "react";

interface PageToolbarProps {
  leftControls?: React.ReactNode;
  rightSlot?: React.ReactNode;
}

/*
PageToolbar is the secondary control strip that sits flush under the
glass nav on dashboard / history. Floats on the ambient bg with no
borders or fill of its own so the nav and toolbar read as one
continuous glass strip; visual separation from page content comes
from the section spacing below it, not a hairline.
*/
export function PageToolbar({ leftControls, rightSlot }: PageToolbarProps): React.JSX.Element | null {
  if (!leftControls && !rightSlot) return null;

  return (
    <div className="mx-auto flex max-w-[1200px] flex-wrap items-center justify-between gap-2 px-4 py-2 sm:gap-3 sm:px-7">
      <div className="flex flex-wrap items-center gap-2 sm:gap-3">{leftControls}</div>
      {rightSlot && <div className="flex shrink-0 items-center">{rightSlot}</div>}
    </div>
  );
}
