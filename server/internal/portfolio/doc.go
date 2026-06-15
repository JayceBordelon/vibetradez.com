/*
Package portfolio runs the v2 autonomous portfolio manager.

This package replaces the two-stage options picker (internal/trades 9:25
selection plus internal/execagent 9:30 at-open agent) with a single daily
agent that decides what to do with the account from scratch: buy equity,
buy options, trim, sell, or hold cash, sizing each move itself. Positions
are held across days. The mandate is to grow account value with an
aggressive risk tolerance, benchmarked against buy-and-hold SPY.

See docs/DESIGN-portfolio-manager-v2.md for the full design.

Design constraints (mirrored from execagent, the package this supersedes):

  - The tool surface is the SECURITY BOUNDARY between the model and real
    money. The settled-cash rule and sell validation live in guards.go and
    are enforced at the tool layer (tools.go), never only in the prompt. A
    jailbroken or buggy model output must not be able to spend unsettled
    cash or oversell a position.
  - The model has full discretion over allocation: there is no stock/option
    split, no per-name or per-order cap, and no drawdown breaker. The gates
    above are broker compliance and correctness, not a risk policy.
  - This package is decoupled from the broker (exec), the market-data
    client (schwab), and persistence (store) via the narrow PortfolioReader
    and PortfolioExecutor interfaces, so it compiles and unit-tests on its
    own. Concrete adapters that bind these to the real Schwab client and
    exec.Service are wired in cmd/scanner/main.go (see task 4).
  - "hold" (do nothing) is a first-class outcome, not silence. A do-nothing
    day is a recorded decision with a reason.
*/
package portfolio
