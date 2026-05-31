import { Separator } from "@/components/ui/separator";

/*
Numbered section used by the Terms and Privacy MDX content. Each section
carries an explicit id so the sticky table-of-contents anchors keep
working; scroll-mt offsets it below the fixed nav.
*/
export function Section({ id, num, title, children }: { id: string; num: number; title: string; children: React.ReactNode }) {
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
