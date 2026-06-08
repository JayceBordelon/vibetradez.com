#!/usr/bin/env node
//
// UI-audit screenshot capture.
//
// Drives headless Chromium over every route in routes.mjs at each viewport
// (desktop 1440x900, mobile 390x844) in both light and dark themes, writing
// full-page PNGs into the output directory. Four shots per page:
//   <slug>__<viewport>__<theme>.png
//
// Theme is forced by seeding localStorage["theme"] before any page script
// runs (next-themes, attribute="class", default storageKey "theme") and by
// emulating prefers-color-scheme to match. Motion is reduced so captures
// land on the settled frame rather than mid-animation.
//
// Usage (normally invoked by run.sh, which resolves dynamic routes):
//   BASE_URL=http://localhost:3005 OUT_DIR=../../docs/ui-audits/2026-06-08 \
//   AUDIT_ROUTES='[{"slug":"dashboard","path":"/dashboard",...}]' \
//   node capture.mjs
//
// Env:
//   BASE_URL       default http://localhost:3005
//   OUT_DIR        required: where PNGs are written (created if missing)
//   AUDIT_ROUTES   optional JSON array overriding routes.mjs (resolved paths)
//   SETTLE_MS      default 1800: extra wait after networkidle for charts

import { mkdir } from "node:fs/promises";
import { chromium, devices } from "playwright";
import { ROUTES as STATIC_ROUTES, VIEWPORTS, THEMES } from "./routes.mjs";

const BASE_URL = (process.env.BASE_URL || "http://localhost:3005").replace(/\/$/, "");
const OUT_DIR = process.env.OUT_DIR;
const SETTLE_MS = Number(process.env.SETTLE_MS || 1800);
// Full-page audit shots are stored as JPEG so a four-shot-per-page audit
// stays a few MB (committed to history per PR) instead of ~50MB of PNG.
// Quality 82 keeps body text and chart labels legible.
const JPEG_QUALITY = Number(process.env.JPEG_QUALITY || 82);
// 1x keeps each audit a few MB (it is committed per PR). Bump to 2 via env
// for a one-off high-DPI capture when pixel-peeping a specific glitch.
const SCALE = Number(process.env.SCALE || 1);

if (!OUT_DIR) {
  console.error("OUT_DIR is required");
  process.exit(1);
}

const ROUTES = process.env.AUDIT_ROUTES ? JSON.parse(process.env.AUDIT_ROUTES) : STATIC_ROUTES;

await mkdir(OUT_DIR, { recursive: true });

// Step the viewport from top to bottom and back, pausing at each step, so
// every scroll-reveal IntersectionObserver fires and count-up animations
// finish before a fullPage capture. Without this the landing chapters stay
// at opacity:0 and the screenshot shows large blank gaps.
async function autoScroll(page) {
  await page.evaluate(async () => {
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    const step = Math.max(300, Math.floor(window.innerHeight * 0.8));
    for (let y = 0; y <= document.body.scrollHeight; y += step) {
      window.scrollTo(0, y);
      await sleep(120);
    }
    window.scrollTo(0, document.body.scrollHeight);
    await sleep(200);
    window.scrollTo(0, 0);
  });
}

const browser = await chromium.launch();
const results = [];

for (const theme of THEMES) {
  for (const vp of VIEWPORTS) {
    const context = await browser.newContext({
      viewport: { width: vp.width, height: vp.height },
      deviceScaleFactor: SCALE,
      colorScheme: theme,
      reducedMotion: "reduce",
      isMobile: vp.name === "mobile",
      hasTouch: vp.name === "mobile",
      userAgent: vp.name === "mobile" ? devices["iPhone 14"].userAgent : undefined,
    });
    // Seed next-themes before any app script runs so there is no
    // light->dark flash and the captured frame is the chosen theme.
    await context.addInitScript((t) => {
      try {
        localStorage.setItem("theme", t);
      } catch {}
    }, theme);

    const page = await context.newPage();
    for (const route of ROUTES) {
      if (route.path.includes(":")) {
        console.warn(`! skipping unresolved route ${route.slug} (${route.path})`);
        continue;
      }
      const url = BASE_URL + route.path;
      const file = `${OUT_DIR}/${route.slug}__${vp.name}__${theme}.jpg`;
      try {
        const resp = await page.goto(url, { waitUntil: "networkidle", timeout: 45000 });
        const status = resp ? resp.status() : 0;
        // Let charts (Recharts / lightweight-charts) and lazy content settle.
        await page.waitForTimeout(SETTLE_MS);
        // The landing page (and any scroll-reveal section) renders chapters at
        // opacity:0 until their IntersectionObserver fires. A naive fullPage
        // shot captures them still hidden, leaving huge blank gaps. Step the
        // viewport down the whole page to trip every observer, settle the
        // count-up / reveal animations, then return to top before capturing.
        await autoScroll(page);
        await page.waitForTimeout(600);
        await page.screenshot({ path: file, fullPage: true, type: "jpeg", quality: JPEG_QUALITY });
        results.push({ slug: route.slug, vp: vp.name, theme, status, file });
        console.log(`ok  ${route.slug.padEnd(12)} ${vp.name.padEnd(7)} ${theme.padEnd(5)} ${status} -> ${file}`);
      } catch (err) {
        results.push({ slug: route.slug, vp: vp.name, theme, status: "ERR", file, error: String(err.message || err) });
        console.error(`ERR ${route.slug.padEnd(12)} ${vp.name.padEnd(7)} ${theme.padEnd(5)} ${err.message || err}`);
      }
    }
    await context.close();
  }
}

await browser.close();

const failures = results.filter((r) => r.status === "ERR" || (typeof r.status === "number" && r.status >= 400));
console.log(`\n${results.length} captures, ${failures.length} problem(s).`);
if (failures.length) {
  for (const f of failures) console.log(`  ${f.slug} ${f.vp} ${f.theme}: ${f.status} ${f.error || ""}`);
  process.exitCode = 2;
}
