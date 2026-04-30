import { ScrollText } from "lucide-react";
import type { Metadata } from "next";

import { Separator } from "@/components/ui/separator";

const OG_IMAGE = "/opengraph-image";

export const metadata: Metadata = {
  title: "Terms of Service",
  description: "VibeTradez terms of service, risk disclosures, and legal disclaimers for AI-generated options trade suggestions.",
  openGraph: {
    title: "VibeTradez | Terms of Service",
    description: "VibeTradez terms of service, risk disclosures, and legal disclaimers for AI-generated options trade suggestions.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Terms of Service",
    images: [OG_IMAGE],
  },
};

const sections = [
  { id: "experimental", title: "Experimental Nature of This Service" },
  { id: "not-advice", title: "Not Financial Advice" },
  { id: "risk", title: "Significant Risk Disclosure" },
  { id: "hypothetical", title: "Hypothetical Performance" },
  { id: "auto-execution", title: "Auto-Execution Pipeline" },
  { id: "data", title: "Data Sources & Accuracy" },
  { id: "warranty", title: "No Warranty & Limitation of Liability" },
  { id: "contact", title: "Contact" },
];

export default function TermsPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
      <div className="mb-10 flex items-start gap-3">
        <div className="rounded-md border bg-card p-2 shadow-sm">
          <ScrollText className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Terms of Service</h1>
          <p className="mt-1 text-sm text-muted-foreground">Last updated: April 2026</p>
        </div>
      </div>

      <div className="grid gap-10 lg:grid-cols-[200px_1fr]">
        {/* Sticky TOC (desktop only) */}
        <aside className="hidden lg:block">
          <nav className="sticky top-24">
            <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">On this page</div>
            <ul className="mt-3 space-y-1.5 text-sm">
              {sections.map((s, i) => (
                <li key={s.id}>
                  <a href={`#${s.id}`} className="block py-1 text-muted-foreground transition-colors hover:text-foreground">
                    {i + 1}. {s.title}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
        </aside>

        {/* Long-form content */}
        <article className="prose-terms min-w-0">
          <Section id="experimental" num={1} title="Experimental Nature of This Service">
            <p>
              VibeTradez is an <strong>experimental, educational project</strong> that generates AI-powered options trade suggestions. The platform is provided on an &quot;as-is&quot; basis for
              informational and entertainment purposes only. It is not a registered investment advisory service, broker-dealer, or financial institution.
            </p>
            <p>
              All trade ideas presented on this platform are machine-generated suggestions, not recommendations. They are produced by a single large language model (Claude) running with a fixed prompt
              and a fixed toolset (Schwab market data plus web search) over publicly available sentiment data and live market information. None of these outputs have been reviewed, verified, or
              endorsed by any licensed financial professional.
            </p>
          </Section>

          <Section id="not-advice" num={2} title="Not Financial Advice">
            <p>
              <strong>Nothing on VibeTradez constitutes financial, investment, tax, or legal advice.</strong> The trade suggestions, performance analytics, and any other content should not be
              interpreted as a recommendation to buy, sell, or hold any security or financial instrument.
            </p>
            <p>
              You should always consult with a qualified, licensed financial advisor before making any investment decisions. Do not rely on this platform as a substitute for professional financial
              guidance.
            </p>
          </Section>

          <Section id="risk" num={3} title="Significant Risk Disclosure">
            <p>
              <strong>Options trading involves substantial risk of loss and is not suitable for all investors.</strong> You can lose your entire investment, and in some cases, losses can exceed your
              initial investment. Short-dated options (0 to 7 DTE), which are the focus of this platform, are especially volatile and carry elevated risk of total loss.
            </p>
            <p>
              Past performance displayed on this platform, whether hypothetical, simulated, or based on actual market data, does not guarantee future results. The P&amp;L figures shown are estimates
              based on option mark prices at market open and close, and may not reflect actual executable prices due to bid-ask spreads, liquidity, and market microstructure.
            </p>
            <p>By using this platform, you acknowledge that you understand these risks and accept full responsibility for any trading decisions you make.</p>
          </Section>

          <Section id="hypothetical" num={4} title="Hypothetical Performance">
            <p>
              Performance metrics for picks #2 through #10 on VibeTradez are <strong>hypothetical</strong>. They assume that each suggested trade was entered at the estimated market open price and
              exited at the closing mark price, with one contract per trade. No actual orders are placed for these picks.
            </p>
            <p>
              Hypothetical results have inherent limitations. Unlike actual trading, simulated results do not account for slippage, commissions, margin requirements, the impact of liquidity, or the
              psychological factors of real capital at risk.
            </p>
          </Section>

          <Section id="auto-execution" num={5} title="Auto-Execution Pipeline">
            <p>
              The rank-1 pick of each trading day is automatically executed by the platform. By default this is a <strong>paper trade</strong> (a synthetic order that fills at the live Schwab option
              mark in a simulated environment, with no real money or real positions). The operator has the option to flip the system into <strong>live mode</strong>, in which a real order is placed
              against the operator&apos;s personal Schwab brokerage account using the Schwab Trader API. Live mode is configured server-side via an environment variable; subscribers cannot enable or
              disable it.
            </p>
            <p>
              The auto-execution pipeline operates with three hard guardrails: (1) a price cap of $5/share (= $500 of capital exposure per contract); (2) a mandatory close at 3:55 PM ET (12:55 PM ET
              on half-trading days) regardless of P&amp;L; and (3) a single contract per trade, hardcoded at the package level.
            </p>
            <p>
              Trades originating from the auto-execution pipeline surface on the dashboard with a clearly-labeled badge indicating PAPER or LIVE. The badge state always reflects the actual mode the
              order was placed in, never inferred or extrapolated.
            </p>
          </Section>

          <Section id="data" num={6} title="Data Sources & Accuracy">
            <p>
              Trade suggestions are generated using data from third-party sources including StockTwits, Yahoo Finance, Finviz, SEC EDGAR, the Anthropic Claude API, and the Schwab Market Data API.
              While we strive for accuracy, we make no guarantees regarding the completeness, reliability, or timeliness of any data presented.
            </p>
            <p>Market data, option prices, and stock quotes may be delayed or inaccurate. Always verify prices with your broker before placing any trades.</p>
          </Section>

          <Section id="warranty" num={7} title="No Warranty & Limitation of Liability">
            <p>
              VibeTradez is provided without warranty of any kind, express or implied. The creator of this platform shall not be held liable for any financial losses, damages, or other consequences
              arising from your use of or reliance on the information provided.
            </p>
            <p>This platform may experience downtime, data inaccuracies, or system errors. Trade suggestions may be delayed, missing, or incorrect. Use the platform at your own risk.</p>
          </Section>

          <Section id="contact" num={8} title="Contact">
            <p>
              This project is built and maintained by{" "}
              <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer">
                Jayce Bordelon
              </a>
              . For questions or concerns, reach out via the contact information on the personal site.
            </p>
          </Section>

          <div className="mt-12">
            <a href="#top" className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline underline-offset-2 hover:text-foreground sm:min-h-0">
              ↑ Back to top
            </a>
          </div>
        </article>
      </div>
    </div>
  );
}

function Section({ id, num, title, children }: { id: string; num: number; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="mb-10 scroll-mt-24">
      <h2 className="text-xl font-semibold tracking-tight">
        <span className="text-muted-foreground">{num}.</span> {title}
      </h2>
      <Separator className="my-3" />
      {children}
    </section>
  );
}
