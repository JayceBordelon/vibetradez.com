/*
Package portfolio runs the v2 autonomous portfolio manager.

This package replaces the two-stage options picker (internal/trades 9:25
selection plus internal/execagent 9:30 at-open agent) with an autonomous
agent that runs three times a trading day and decides what to do with the
account from scratch: buy options, trim, sell, liquidate leftover stock, or
hold cash, sizing each move itself. It trades options only. Positions are
held across days. The mandate is to grow account value with an aggressive
risk tolerance, benchmarked against buy-and-hold SPY.

See docs/DESIGN-portfolio-manager-v2.md for the full design.

Design constraints (mirrored from execagent, the package this supersedes):

  - The tool surface is the SECURITY BOUNDARY between the model and real
    money. The settled-cash rule and sell validation live in guards.go and
    are enforced at the tool layer (tools.go), never only in the prompt. A
    jailbroken or buggy model output must not be able to spend unsettled
    cash or oversell a position.
  - The model trades options only (equity buys are disabled). Sizing is
    capped at the tool layer: no single option position over ~10% of equity
    (MaxSingleOptionFrac) and no single underlying over ~25%
    (MaxPerUnderlyingFrac). Those caps, the settled-cash rule, and sell
    validation are the gates; nothing else constrains allocation.
  - This package is decoupled from the broker (exec), the market-data
    client (schwab), and persistence (store) via the narrow PortfolioReader
    and PortfolioExecutor interfaces, so it compiles and unit-tests on its
    own. Concrete adapters that bind these to the real Schwab client and
    exec.Service are wired in cmd/scanner/main.go (see task 4).
  - "hold" (do nothing) is a first-class outcome, not silence. A do-nothing
    day is a recorded decision with a reason.
*/
package portfolio
