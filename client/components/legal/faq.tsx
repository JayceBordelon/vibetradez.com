"use client";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

/*
FAQ rendered from MDX: faq.mdx authors the questions/answers as a list of
<QA> entries wrapped in <FaqList>. FaqList owns the single Radix
Accordion; each QA is one collapsible item. Answers are Markdown, so
bold/links render and spacing is automatic (no more squished labels).
*/
function slug(q: string): string {
  return q
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function FaqList({ children }: { children: React.ReactNode }) {
  return (
    <Accordion type="single" collapsible className="lg-card overflow-hidden">
      {children}
    </Accordion>
  );
}

export function QA({ q, children }: { q: string; children: React.ReactNode }) {
  return (
    <AccordionItem value={slug(q)} className="border-b last:border-b-0">
      <AccordionTrigger className="px-5 text-left text-base font-semibold hover:no-underline">{q}</AccordionTrigger>
      <AccordionContent className="px-5 text-[15px] leading-relaxed text-muted-foreground">{children}</AccordionContent>
    </AccordionItem>
  );
}
