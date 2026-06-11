"use client";

import { HelpCircle } from "lucide-react";
import { Children, isValidElement } from "react";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";

/*
FAQ rendered from MDX: faq.mdx authors the questions/answers as a list of
<QA> entries wrapped in <FaqList>. FaqList owns the page header (with a
"N questions" badge counted from the actual QA children, so it can never
drift) and the single Radix Accordion; each QA is one collapsible item.
Answers are Markdown, so bold/links render and spacing is automatic.
*/
function slug(q: string): string {
  return q
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function FaqList({ children }: { children: React.ReactNode }) {
  // Count the real QA elements only (MDX interleaves whitespace text nodes
  // between them), so the header badge always matches what's rendered. We
  // match on the `q` prop rather than `c.type === QA`: across the MDX/RSC
  // client boundary the element type is a client reference, not the QA
  // function identity, so a type check reads as zero.
  const qas = Children.toArray(children).filter((c) => isValidElement(c) && typeof (c.props as { q?: unknown })?.q === "string");
  const count = qas.length;
  // First question starts expanded so the page shows what an answer looks
  // like instead of twelve identical collapsed rows.
  const firstSlug = count > 0 ? slug((qas[0] as React.ReactElement<{ q: string }>).props.q) : undefined;

  return (
    <>
      <div className="mb-8 flex items-start gap-3">
        <div className="lg-control p-2">
          <HelpCircle className="h-5 w-5 text-primary" />
        </div>
        <div>
          <h1 className="term-display text-xl font-extrabold uppercase tracking-tight sm:text-2xl">Frequently Asked Questions</h1>
          <div className="mt-1 flex items-center gap-2">
            <p className="text-sm text-muted-foreground">How VibeTradez works under the hood.</p>
            <Badge variant="secondary" className="text-[11px]">
              {count} questions
            </Badge>
          </div>
        </div>
      </div>

      <Accordion type="single" collapsible defaultValue={firstSlug} className="border-t border-border/60">
        {children}
      </Accordion>
    </>
  );
}

export function QA({ q, children }: { q: string; children: React.ReactNode }) {
  return (
    <AccordionItem value={slug(q)} className="border-b border-border/60 last:border-b-0">
      <AccordionTrigger className="text-left text-base font-semibold hover:no-underline">{q}</AccordionTrigger>
      <AccordionContent className="text-[15px] leading-relaxed text-muted-foreground">{children}</AccordionContent>
    </AccordionItem>
  );
}
