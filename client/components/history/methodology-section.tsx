import { BookOpen } from "lucide-react";

import {
	Accordion,
	AccordionContent,
	AccordionItem,
	AccordionTrigger,
} from "@/components/ui/accordion";
import { Card } from "@/components/ui/card";

export function MethodologySection() {
	return (
		<Card className="p-6">
			<div className="flex items-center gap-2">
				<BookOpen className="h-5 w-5 text-muted-foreground" aria-hidden />
				<h2 className="text-base font-semibold">
					Methodology &amp; Disclaimers
				</h2>
			</div>

			<Accordion type="single" collapsible className="mt-2 w-full">
				<AccordionItem value="pipeline">
					<AccordionTrigger className="text-base font-semibold text-left">
						Trade Selection Pipeline
					</AccordionTrigger>
					<AccordionContent>
						<p className="text-[15px] leading-relaxed text-muted-foreground">
							Every trading day follows a fixed automated pipeline. At
							9:25 AM ET, the system scrapes social-media sentiment from
							r/wallstreetbets and adjacent communities, identifies the
							most-discussed tickers, and pulls live market data from the
							Schwab API.
						</p>
						<p className="mt-3 text-[15px] leading-relaxed text-muted-foreground">
							The same raw sentiment payload is then sent independently to
							two LLMs:{" "}
							<strong className="text-foreground">OpenAI GPT-5.4</strong>{" "}
							and{" "}
							<strong className="text-foreground">
								Anthropic Claude Opus 4.6
							</strong>
							. Both models run the identical analysis prompt and have
							access to the same toolset, including live Schwab quotes,
							the full options chain with greeks, and a built-in web
							search for catalyst verification. Each model independently produces
							its own ranked top 10 picks for the day; neither sees the
							other&apos;s output. This is a true head-to-head comparison
							of two independent strategists working from the same source
							material.
						</p>
						<p className="mt-3 text-[15px] leading-relaxed text-muted-foreground">
							Both pick sets are then unioned into a single list. When
							both models picked the same ticker (a consensus pick) the
							row carries both models&apos; scores and rationales and the
							combined score is the average. When only one model picked a
							ticker the other side&apos;s score is left at zero. Final
							rank is by combined score with consensus picks tie-breaking
							ahead of single-model picks. The All / OpenAI / Claude
							filter in the nav bar lets you slice the day either as the
							merged consensus view or as exactly what one model would
							have produced on its own.
						</p>
						<p className="mt-3 text-[15px] leading-relaxed text-muted-foreground">
							All positions are opened at the market open and closed at
							4:05 PM ET. No trades are held overnight. The model
							identifiers used in production are configurable via the
							OPENAI_MODEL and ANTHROPIC_MODEL environment variables and
							default to the latest production model in each
							provider&apos;s official Go SDK.
						</p>
					</AccordionContent>
				</AccordionItem>

				<AccordionItem value="metrics">
					<AccordionTrigger className="text-base font-semibold text-left">
						Performance Metrics
					</AccordionTrigger>
					<AccordionContent>
						<ul className="space-y-2 text-[15px] leading-relaxed text-muted-foreground">
							<li>
								<strong className="text-foreground">Profit Factor:</strong>{" "}
								ratio of gross winning P&amp;L to gross losing
								P&amp;L. A value above 1.0 means the system is net
								profitable.
							</li>
							<li>
								<strong className="text-foreground">Expectancy:</strong>{" "}
								average dollar P&amp;L per trade, accounting for
								both win rate and average win/loss size. Positive
								expectancy implies long-run profitability.
							</li>
							<li>
								<strong className="text-foreground">Sharpe Ratio:</strong>{" "}
								risk-adjusted return calculated as the mean daily
								P&amp;L divided by its standard deviation. Higher values
								indicate more consistent returns.
							</li>
							<li>
								<strong className="text-foreground">Max Drawdown:</strong>{" "}
								largest peak-to-trough decline in cumulative
								P&amp;L. Reflects the worst observed losing streak.
							</li>
							<li>
								<strong className="text-foreground">
									Return on Capital (ROC):
								</strong>{" "}
								net P&amp;L divided by total capital deployed.
								Measures how efficiently invested capital generates
								returns.
							</li>
						</ul>
					</AccordionContent>
				</AccordionItem>

				<AccordionItem value="pricing">
					<AccordionTrigger className="text-base font-semibold text-left">
						Options Pricing Context
					</AccordionTrigger>
					<AccordionContent>
						<p className="text-[15px] leading-relaxed text-muted-foreground">
							Entry prices are estimated at 9:30 AM using the
							contract&rsquo;s mark price (midpoint of bid/ask). Closing
							prices are recorded at 4:05 PM ET using the same mark-price
							methodology. All P&amp;L figures assume single-contract
							positions (100 shares notional) and do not include commissions
							or slippage.
						</p>
					</AccordionContent>
				</AccordionItem>

				<AccordionItem value="sources">
					<AccordionTrigger className="text-base font-semibold text-left">
						Data Sources
					</AccordionTrigger>
					<AccordionContent>
						<p className="text-[15px] leading-relaxed text-muted-foreground">
							Stock and option prices (bid, ask, mark, greeks, open
							interest, volume) are sourced live from the Schwab
							Market Data API via authenticated OAuth. Sentiment data is
							scraped from Reddit&apos;s public JSON feeds. Both LLMs can
							additionally use a built-in web search tool to verify
							catalysts, earnings dates, and recent news. Every price you
							see in a pick comes from real market data, and the models
							are explicitly instructed never to guess prices.
						</p>
					</AccordionContent>
				</AccordionItem>

				<AccordionItem value="comparison">
					<AccordionTrigger className="text-base font-semibold text-left">
						Model Comparison
					</AccordionTrigger>
					<AccordionContent>
						<p className="text-[15px] leading-relaxed text-muted-foreground">
							The Models page replays the trade history under each
							model&apos;s ranking in isolation. For each day in the
							selected range we take the picks the model originally chose,
							compute their realised P&amp;L from the EOD summaries, and
							aggregate by model. The line chart shows the cumulative P&amp;L
							you would have made by following only OpenAI, only Claude, or
							the combined consensus ranking. The agreement-rate stat shows
							the fraction of dual-scored trades where the two models were
							within one point of each other, which serves as a rough
							gauge of how much the models actually disagree on what
							looks like a good setup.
						</p>
					</AccordionContent>
				</AccordionItem>

				<AccordionItem value="disclaimers">
					<AccordionTrigger className="text-base font-semibold text-left">
						Disclaimers
					</AccordionTrigger>
					<AccordionContent>
						<p className="text-[15px] leading-relaxed text-muted-foreground">
							This dashboard is for informational and educational purposes
							only. Past performance does not guarantee future results. The
							trades shown are generated by an automated system and do not
							constitute financial advice. Options trading involves
							substantial risk and is not suitable for all investors. Always
							do your own research before making investment decisions.
						</p>
					</AccordionContent>
				</AccordionItem>
			</Accordion>
		</Card>
	);
}
