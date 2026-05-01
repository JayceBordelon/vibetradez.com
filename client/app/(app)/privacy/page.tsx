import { Lock } from "lucide-react";
import type { Metadata } from "next";

import { Separator } from "@/components/ui/separator";

const OG_IMAGE = "/opengraph-image";

export const metadata: Metadata = {
  title: "Privacy Policy",
  description: "VibeTradez privacy policy: what data is collected, how it's used, and which third-party processors handle it.",
  openGraph: {
    title: "VibeTradez | Privacy Policy",
    description: "VibeTradez privacy policy: what data is collected, how it's used, and which third-party processors handle it.",
    images: [{ url: OG_IMAGE, width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "VibeTradez | Privacy Policy",
    images: [OG_IMAGE],
  },
};

const sections = [
  { id: "scope", title: "Scope" },
  { id: "what-we-collect", title: "What We Collect" },
  { id: "how-we-use-it", title: "How We Use It" },
  { id: "processors", title: "Third-Party Processors" },
  { id: "no-payment-data", title: "No Payment Data" },
  { id: "retention", title: "Retention & Deletion" },
  { id: "your-rights", title: "Your Rights" },
  { id: "cookies", title: "Cookies" },
  { id: "security", title: "Security" },
  { id: "changes", title: "Changes to This Policy" },
  { id: "contact", title: "Contact" },
];

export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6">
      <div className="mb-10 flex items-start gap-3">
        <div className="lg-control p-2">
          <Lock className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Privacy Policy</h1>
          <p className="mt-1 text-sm text-muted-foreground">Last updated: April 2026</p>
        </div>
      </div>

      <div className="grid gap-10 lg:grid-cols-[200px_1fr]">
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

        <article className="lg-card prose-terms min-w-0 p-6 sm:p-8">
          <Section id="scope" num={1} title="Scope">
            <p>
              This policy describes what data VibeTradez collects from visitors and subscribers, how that data is used, and which third-party processors handle it on our behalf. It applies to the
              vibetradez.com website, the daily picks email, and the auto-execution pipeline operated by the project maintainer.
            </p>
            <p>
              VibeTradez is a one-person side project. There is no analytics SDK, no advertising network, no cross-site tracker, and no third-party data broker involved. The data footprint is
              intentionally small.
            </p>
          </Section>

          <Section id="what-we-collect" num={2} title="What We Collect">
            <p>If you sign in or subscribe, we collect:</p>
            <ul>
              <li>
                <strong>Email address</strong> (from Google OAuth) and the <strong>Google account display name</strong>. Used to send you the daily picks email and to identify you in server logs.
              </li>
              <li>
                <strong>An opaque session token</strong> issued by the auth service after you sign in with Google. Stored as an HTTP-only cookie on vibetradez.com (named <code>vt_session</code>) so
                you stay logged in across visits.
              </li>
              <li>
                <strong>Email-delivery state</strong>: timestamps of when each daily picks email was sent to you, plus your active/unsubscribed status. No open or click tracking pixels.
              </li>
            </ul>
            <p>
              If you do not sign in, the only data we collect is what your browser sends automatically (IP address, user agent, referrer) for the duration of each request, in standard server logs.
            </p>
          </Section>

          <Section id="how-we-use-it" num={3} title="How We Use It">
            <p>The data above is used for exactly these things, and nothing else:</p>
            <ul>
              <li>Authenticating you on subsequent visits to the dashboard.</li>
              <li>Sending you the daily picks email at 9:25 AM ET, the EOD summary at 4:05 PM ET, the Friday weekly digest, and occasional one-shot announcements about feature changes.</li>
              <li>Keeping subscription state correct (active vs unsubscribed).</li>
              <li>Diagnosing operational issues (e.g. understanding why a particular request 500'd, who saw a particular bug).</li>
            </ul>
            <p>We do not sell, rent, share, or trade your data. We do not use it to train any machine learning model. We do not run targeted advertising.</p>
          </Section>

          <Section id="processors" num={4} title="Third-Party Processors">
            <p>VibeTradez relies on a small number of external services to function. Each is a standard, contracted data processor handling a narrow slice of the workload:</p>
            <ul>
              <li>
                <strong>Google OAuth</strong> (Google LLC) authenticates sign-ins. Google sees your email address and grants us read-only access to your basic profile (name + email + avatar URL).
              </li>
              <li>
                <strong>Resend</strong> (Resend, Inc.) delivers transactional email. They process recipient email addresses, the email body, and delivery metadata (sent/bounced/etc.) on our behalf.
              </li>
              <li>
                <strong>Anthropic</strong> (Anthropic PBC) powers the trade picker. We send Claude the daily market signals payload and our standard analysis prompt; we do <strong>not</strong> send
                Claude any subscriber data, account data, or personally-identifying information.
              </li>
              <li>
                <strong>Schwab</strong> (Charles Schwab &amp; Co., Inc.) provides market data (quotes, option chains) and, in live trading mode, executes orders against the project maintainer&apos;s
                personal brokerage account. Schwab does not see any subscriber data; the Schwab integration is one-way (we read from them, and the maintainer writes to them).
              </li>
              <li>
                <strong>Digital Ocean</strong> (DigitalOcean LLC) hosts the application server and the managed Postgres database. They process all data above as the underlying infrastructure provider.
              </li>
              <li>
                <strong>GoDaddy</strong> (GoDaddy.com, LLC) is the domain registrar. They see DNS lookups for vibetradez.com.
              </li>
            </ul>
          </Section>

          <Section id="no-payment-data" num={5} title="No Payment Data">
            <p>
              VibeTradez is and always will be free. There is no subscription tier, no premium upgrade, no paywall. We do not process credit cards, bank accounts, or any other payment information.
              Stripe, PayPal, and similar payment processors are not part of this stack.
            </p>
          </Section>

          <Section id="retention" num={6} title="Retention &amp; Deletion">
            <p>
              Subscriber records (email + name + active status) are retained for the lifetime of the project. If you unsubscribe, your row remains in the database with <code>active = false</code> so
              we can honor the unsubscribe across re-signups; we do not re-add you automatically.
            </p>
            <p>Server logs (request metadata) are retained for approximately 30 days for operational debugging, then rotated out.</p>
            <p>If you want your data fully deleted (not just deactivated), email the maintainer using the contact info below and the row will be hard-deleted from the subscribers table.</p>
          </Section>

          <Section id="your-rights" num={7} title="Your Rights">
            <p>
              Depending on where you live, you may have legal rights to access, correct, port, or delete the data we hold about you. We honor reasonable requests regardless of jurisdiction; just email
              the maintainer.
            </p>
            <p>You can also unsubscribe from all email at any time by clicking the unsubscribe link at the bottom of any email, or via the dashboard account menu.</p>
          </Section>

          <Section id="cookies" num={8} title="Cookies">
            <p>VibeTradez sets exactly one cookie:</p>
            <ul>
              <li>
                <code>vt_session</code> &mdash; HTTP-only, SameSite=Lax, Secure (in production). Holds the opaque access token issued by the auth service. Required for sign-in to work. Cleared when
                you sign out, expires after 30 days of inactivity.
              </li>
            </ul>
            <p>No analytics cookies, no ad cookies, no fingerprinting.</p>
          </Section>

          <Section id="security" num={9} title="Security">
            <p>
              All traffic to vibetradez.com is served over HTTPS via Let&apos;s Encrypt. The Postgres database is on a private network only the application server can reach. The Anthropic and Schwab
              API keys are stored as environment variables on the server, never in client-side code or version control. Subscriber-list emails are sent via Resend with subscriber addresses BCC&apos;d
              so recipients don&apos;t see each other.
            </p>
            <p>
              That said: this is a side project, not a regulated financial institution. Treat the security posture accordingly. The data we hold is minimal by design specifically because we don&apos;t
              want to be a target.
            </p>
          </Section>

          <Section id="changes" num={10} title="Changes to This Policy">
            <p>
              If this policy changes materially (e.g. we add a new third-party processor, or change what data we collect), we will email subscribers via the rollout system before the change takes
              effect, and bump the &quot;Last updated&quot; date at the top of this page.
            </p>
          </Section>

          <Section id="contact" num={11} title="Contact">
            <p>
              This project is built and maintained by{" "}
              <a href="https://jaycebordelon.com" target="_blank" rel="noopener noreferrer">
                Jayce Bordelon
              </a>
              . For privacy questions, data deletion requests, or anything else policy-related, reach out via the contact information on the personal site.
            </p>
          </Section>

          <div className="mt-12">
            <a href="#top" className="inline-flex min-h-11 items-center text-sm text-muted-foreground underline underline-offset-2 hover:text-foreground sm:min-h-0">
              Back to top
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
