// Regenerates the raster brand assets (favicon PNG + OpenGraph cards) from
// the current theme. Re-run after a theme change:  node scripts/gen-brand-assets.mjs
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import sharp from "sharp";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

// Theme colors (designbyte): mint primary on near-black, emerald accent.
const MINT = "#51F0A8";
const EMERALD = "#10B981";
const BG = "#0A0A0A";
const PANEL = "#101512";
const FG = "#F5F7F6";
const MUTED = "#9BA3A0";

// 1) Favicon raster from the (green) SVG.
const iconSvg = readFileSync(join(root, "app/icon.svg"));
await sharp(iconSvg, { density: 384 }).resize(512, 512).png().toFile(join(root, "app/icon.png"));

// Green bar-chart motif (echoes the favicon), drawn at an offset.
function bars(ox, oy, scale, opacity) {
  const b = (x, y, w, h) => `<rect x="${ox + x * scale}" y="${oy + y * scale}" width="${w * scale}" height="${h * scale}" rx="${6 * scale}" fill="${MINT}" opacity="${opacity}"/>`;
  return [b(0, 150, 44, 96), b(92, 92, 44, 154), b(184, 30, 44, 216)].join("");
}

function ogSvg(title, subtitle) {
  return `<svg width="1200" height="630" viewBox="0 0 1200 630" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="accent" x1="0" y1="0" x2="1" y2="0">
      <stop stop-color="${MINT}"/><stop offset="1" stop-color="${EMERALD}"/>
    </linearGradient>
    <radialGradient id="glow" cx="0.82" cy="0.2" r="0.7">
      <stop stop-color="${MINT}" stop-opacity="0.18"/><stop offset="1" stop-color="${MINT}" stop-opacity="0"/>
    </radialGradient>
  </defs>
  <rect width="1200" height="630" fill="${BG}"/>
  <rect width="1200" height="630" fill="url(#glow)"/>
  <rect x="0" y="0" width="1200" height="10" fill="url(#accent)"/>
  <g transform="translate(700,150)">${bars(0, 0, 1.1, 0.16)}</g>
  <g font-family="'Plus Jakarta Sans','Helvetica Neue',Arial,sans-serif">
    <g transform="translate(80,96)">
      <rect width="56" height="56" rx="16" fill="url(#accent)"/>
      <g fill="${BG}">
        <rect x="14" y="33" width="8" height="13" rx="2"/><rect x="26" y="24" width="8" height="22" rx="2"/><rect x="38" y="15" width="8" height="31" rx="2"/>
      </g>
      <text x="72" y="40" font-size="34" font-weight="800" fill="${FG}">Vibe<tspan fill="${MINT}">Tradez</tspan></text>
    </g>
    <text x="80" y="330" font-size="68" font-weight="800" fill="${FG}">${title}</text>
    <text x="80" y="392" font-size="32" font-weight="500" fill="${MUTED}">${subtitle}</text>
    <g transform="translate(80,500)">
      <rect width="200" height="44" rx="22" fill="${PANEL}" stroke="${MINT}" stroke-opacity="0.35"/>
      <circle cx="26" cy="22" r="6" fill="${MINT}"/>
      <text x="44" y="29" font-size="17" font-weight="700" fill="${MINT}" letter-spacing="1">LIVE · REAL MONEY</text>
    </g>
  </g>
</svg>`;
}

const pages = {
  "landing.png": ["An AI runs real money.", "One model. Zero humans. Stocks, options, or cash."],
  "dashboard.png": ["The live book.", "Holdings, moves, and the equity curve against SPY."],
  "faq.png": ["FAQ", "How VibeTradez works under the hood."],
  "privacy.png": ["Privacy Policy", "What we collect and how it is used."],
  "terms.png": ["Terms of Service", "Risk disclosures and disclaimers."],
  "history.png": ["Track record", "Every session, benchmarked to buy-and-hold SPY."],
};

for (const [file, [title, subtitle]] of Object.entries(pages)) {
  await sharp(Buffer.from(ogSvg(title, subtitle))).png().toFile(join(root, "public/og", file));
}

console.log("Regenerated app/icon.png and", Object.keys(pages).length, "OG images.");
