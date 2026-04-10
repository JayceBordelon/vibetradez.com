"use client";

import { useEffect, useRef, useState } from "react";

export function useCountUp(target: number, durationMs = 600): number {
	const previousValue = useRef<number>(target);
	const [displayed, setDisplayed] = useState<number>(target);

	useEffect(() => {
		if (!Number.isFinite(target)) {
			previousValue.current = target;
			setDisplayed(target);
			return;
		}

		const start = previousValue.current;
		const end = target;

		if (start === end) {
			return;
		}

		const t0 =
			typeof performance !== "undefined" ? performance.now() : Date.now();
		let rafId = 0;

		const tick = (now: number) => {
			const elapsed = now - t0;
			const progress = Math.min(Math.max(elapsed / durationMs, 0), 1);
			const eased = 1 - (1 - progress) ** 3;
			const value = start + (end - start) * eased;
			setDisplayed(value);

			if (progress < 1) {
				rafId = requestAnimationFrame(tick);
			} else {
				previousValue.current = end;
			}
		};

		rafId = requestAnimationFrame(tick);

		return () => {
			cancelAnimationFrame(rafId);
		};
	}, [target, durationMs]);

	return displayed;
}
