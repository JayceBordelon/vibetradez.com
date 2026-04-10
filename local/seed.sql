-- VibeTradez local development seed data
--
-- Creates the schema (matching internal/store/store.go) and inserts ~2 weeks
-- of realistic test trades with corresponding EOD summaries.
--
-- This file runs automatically on first Postgres boot via the
-- /docker-entrypoint-initdb.d mount in docker-compose.local.yml.

-- ─── Schema (mirrors internal/store/store.go migrate()) ────────────────────

CREATE TABLE IF NOT EXISTS trades (
    id SERIAL PRIMARY KEY,
    date TEXT NOT NULL,
    symbol TEXT NOT NULL,
    contract_type TEXT NOT NULL,
    strike_price DOUBLE PRECISION NOT NULL,
    expiration TEXT NOT NULL,
    dte INTEGER NOT NULL,
    estimated_price DOUBLE PRECISION NOT NULL,
    thesis TEXT NOT NULL DEFAULT '',
    sentiment_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    current_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    target_price DOUBLE PRECISION NOT NULL DEFAULT 0,
    stop_loss DOUBLE PRECISION NOT NULL DEFAULT 0,
    profit_target DOUBLE PRECISION NOT NULL DEFAULT 0,
    risk_level TEXT NOT NULL DEFAULT '',
    catalyst TEXT NOT NULL DEFAULT '',
    mention_count INTEGER NOT NULL DEFAULT 0,
    rank INTEGER NOT NULL DEFAULT 0,
    gpt_score INTEGER NOT NULL DEFAULT 0,
    gpt_rationale TEXT NOT NULL DEFAULT '',
    claude_score INTEGER NOT NULL DEFAULT 0,
    claude_rationale TEXT NOT NULL DEFAULT '',
    combined_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_trades_date ON trades(date);

CREATE TABLE IF NOT EXISTS summaries (
    id SERIAL PRIMARY KEY,
    date TEXT NOT NULL,
    symbol TEXT NOT NULL,
    contract_type TEXT NOT NULL,
    strike_price DOUBLE PRECISION NOT NULL,
    expiration TEXT NOT NULL,
    entry_price DOUBLE PRECISION NOT NULL,
    closing_price DOUBLE PRECISION NOT NULL,
    stock_open DOUBLE PRECISION NOT NULL,
    stock_close DOUBLE PRECISION NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_summaries_date ON summaries(date);

CREATE TABLE IF NOT EXISTS subscribers (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    unsubscribed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_subscribers_active ON subscribers(active);

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id SERIAL PRIMARY KEY,
    provider TEXT NOT NULL UNIQUE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Wipe any existing data so re-running this file is idempotent.
TRUNCATE trades, summaries, subscribers RESTART IDENTITY;

-- ─── Subscribers (handful of test accounts) ───────────────────────────────

INSERT INTO subscribers (email, name, active) VALUES
    ('local@vibetradez.local', 'Local Test', true),
    ('demo@example.com', 'Demo User', true),
    ('jayce@vibetradez.local', 'Jayce', true);

-- ─── Trades + Summaries ────────────────────────────────────────────────────
--
-- Two weeks of trade history (10 trading days) with 10 ranked picks each.
-- Mix of winners and losers across CALL/PUT, varied risk levels, varied
-- sentiment. The most recent date is fully populated WITHOUT summaries
-- (simulating a "morning picks" pre-EOD state). All other days have full
-- EOD summaries.

DO $$
DECLARE
    -- 10 trading days, ending today, skipping weekends.
    -- We compute these in SQL so the seed always reflects "the last 10 weekdays".
    today_date DATE := CURRENT_DATE;
    d DATE;
    i INT;
    weekday_count INT := 0;
    pick_idx INT;

    -- Pool of realistic-looking tickers
    symbols TEXT[] := ARRAY['NVDA','TSLA','AAPL','AMD','META','SPY','QQQ','MSFT','GOOGL','AMZN','PLTR','COIN','GME','AMC','HOOD','SOFI','RIVN','LCID','SHOP','UBER','SNAP','BABA','NFLX','DIS','BAC','JPM','XOM','CVX','BA','F'];
    contract_types TEXT[] := ARRAY['CALL','CALL','CALL','PUT','PUT','CALL','CALL','PUT','CALL','CALL'];
    risk_levels TEXT[] := ARRAY['LOW','LOW','MEDIUM','MEDIUM','MEDIUM','MEDIUM','HIGH','HIGH','MEDIUM','HIGH'];
    catalysts TEXT[] := ARRAY[
        'Earnings report Friday morning',
        'AI keynote scheduled for tomorrow',
        'FDA approval decision pending',
        'Fed rate decision Wednesday',
        'Major product launch event',
        'Analyst day presentation',
        'Short-interest squeeze potential',
        'Sector rotation tailwind',
        'Options expiry pinning',
        'Insider buying activity'
    ];
    theses TEXT[] := ARRAY[
        'Strong technical setup with clean breakout above 50-day MA. Volume confirms momentum and the implied volatility is reasonably priced relative to historical levels.',
        'Bullish flag forming on the daily chart. Sentiment data shows accelerating mention count on r/wallstreetbets with positive bias.',
        'Mean-reversion play targeting the lower bollinger band. Stock is oversold on RSI and the catalyst risk is well-defined into the close.',
        'Directional bet on weekly options to capture the post-event move. Greeks favor the buyer with theta risk minimized over the holding period.',
        'High-conviction sentiment shift detected. Multiple bullish posts in the last 24 hours with elevated comment engagement.',
        'Volatility crush trade post-earnings. The contract is priced for a larger move than what historical realized vol suggests.',
        'Sector momentum setup. Peer stocks have rallied on similar catalysts and this name is lagging the move.',
        'Defined-risk speculation on a low-probability but high-reward scenario. Sized appropriately for the conviction level.',
        'Short-dated gamma play. The contract has favorable delta exposure for the expected move into tomorrow.',
        'Hedge against broader market exposure. Negative correlation expected if SPY breaks recent support.'
    ];

    sym TEXT;
    ctype TEXT;
    rlevel TEXT;
    cat TEXT;
    thesis TEXT;
    strike DOUBLE PRECISION;
    stock_price DOUBLE PRECISION;
    estimated DOUBLE PRECISION;
    target DOUBLE PRECISION;
    sentiment DOUBLE PRECISION;
    mentions INT;

    -- For summaries
    entry_p DOUBLE PRECISION;
    closing_p DOUBLE PRECISION;
    stock_open DOUBLE PRECISION;
    stock_close DOUBLE PRECISION;
    pnl_pct DOUBLE PRECISION;
    is_today BOOLEAN;

    -- Dual-model scoring
    gpt_s INT;
    claude_s INT;
    combined DOUBLE PRECISION;
    gpt_rat TEXT;
    claude_rat TEXT;
    gpt_rationales TEXT[] := ARRAY[
        'Highest conviction pick of the day. Sentiment is leaning hard into this name and the option chain shows favorable bid/ask spreads with healthy open interest.',
        'Strong setup but the catalyst window is tight. Sized smaller because the move could happen pre-market and reduce our edge.',
        'Multiple confirming signals: WSB mention velocity is up 3x, technicals are clean, and IV is reasonable for the move we expect.',
        'Decent risk/reward but not a top-tier setup. Including in the list because the broader sector tailwind supports the thesis.',
        'Speculative tail-risk play. Cheap premium and the upside scenario is asymmetric — comfortable losing the entire premium here.',
        'Mean-reversion candidate. The stock is oversold on RSI and the option premium prices in more downside than I think will materialize.',
        'Earnings volatility play. The expected move priced in is smaller than the historical move on similar setups for this name.',
        'Sentiment-driven momentum. WSB is loud on this name and the contract benefits from short-dated gamma into the catalyst.',
        'Hedge against broader exposure. Negative beta to SPY makes this useful as a portfolio protection layer for the day.',
        'Lowest-ranked pick. Including for diversification but conviction is the weakest of the day; would skip if forced to take fewer.'
    ];
    claude_rationales TEXT[] := ARRAY[
        'Confirmed: option chain pricing and underlying technical setup match GPT thesis. Greeks favor the direction with theta risk well managed.',
        'Agree on direction but flag IV percentile at 88th — there is real vega risk if the catalyst lands soft. Score reflects that downside.',
        'Strong technical case is real, but I cannot independently verify the WSB mention velocity claim without the raw data. Discounting slightly.',
        'Moderate conviction. Sector tailwind argument is sound but the specific name has weaker relative strength than peers cited.',
        'Cheap premium claim verified, but the asymmetric upside is conditioned on a specific catalyst date I could not confirm via search.',
        'RSI oversold confirmed. However, mean reversion in trending markets is a frequent trap — would rate higher if there were a clearer reversal signal.',
        'Earnings setup is real and the implied move does look low vs the realized history. High-confidence agreement with the analyzer.',
        'Sentiment momentum is observable but the specific contract greeks are mediocre — gamma exposure is fine, theta drag is steeper than implied.',
        'Hedge thesis is sound. Negative correlation to SPY is well established for this name on the timeframe.',
        'Marginal pick. Liquidity is thin enough that even a small fill could move the mark; would not take this in size.'
    ];
BEGIN
    -- Walk back from today, picking weekdays
    d := today_date;
    WHILE weekday_count < 10 LOOP
        IF EXTRACT(DOW FROM d) NOT IN (0, 6) THEN
            weekday_count := weekday_count + 1;
            is_today := (weekday_count = 1);

            FOR pick_idx IN 1..10 LOOP
                -- Deterministic but varied selection
                sym := symbols[1 + ((pick_idx * 7 + weekday_count * 3) % array_length(symbols, 1))];
                ctype := contract_types[1 + ((pick_idx + weekday_count) % 10)];
                rlevel := risk_levels[1 + ((pick_idx + weekday_count * 2) % 10)];
                cat := catalysts[1 + ((pick_idx + weekday_count) % 10)];
                thesis := theses[1 + ((pick_idx * 3 + weekday_count) % 10)];

                -- Pseudo-random but stable price levels
                stock_price := 50 + ((ascii(substr(sym, 1, 1)) * 7 + pick_idx * 13) % 350);
                strike := round((stock_price + (CASE WHEN ctype = 'CALL' THEN 5 ELSE -5 END) + ((pick_idx * 3) % 15) - 7)::numeric, 0);
                estimated := round((0.50 + ((pick_idx * 17 + weekday_count * 5) % 175) / 100.0)::numeric, 2);
                target := round((stock_price + (CASE WHEN ctype = 'CALL' THEN 8 ELSE -8 END))::numeric, 2);
                sentiment := round((-0.5 + ((pick_idx * 11 + weekday_count * 7) % 150) / 100.0)::numeric, 2);
                mentions := 50 + ((pick_idx * 23 + weekday_count * 11) % 800);

                -- Dual-model scoring: roughly correlated with rank (rank 1
                -- gets highest scores), with a small amount of deterministic
                -- noise so the two models occasionally disagree.
                gpt_s := GREATEST(1, LEAST(10, (11 - pick_idx) + (((pick_idx * 7 + weekday_count * 3) % 3) - 1)));
                claude_s := GREATEST(1, LEAST(10, (11 - pick_idx) + (((pick_idx * 11 + weekday_count * 5) % 5) - 2)));
                combined := (gpt_s + claude_s) / 2.0;
                gpt_rat := gpt_rationales[pick_idx];
                claude_rat := claude_rationales[pick_idx];

                INSERT INTO trades (
                    date, symbol, contract_type, strike_price, expiration, dte,
                    estimated_price, thesis, sentiment_score, current_price,
                    target_price, stop_loss, profit_target, risk_level,
                    catalyst, mention_count, rank,
                    gpt_score, gpt_rationale, claude_score, claude_rationale, combined_score
                ) VALUES (
                    to_char(d, 'YYYY-MM-DD'),
                    sym,
                    ctype,
                    strike,
                    to_char(d + ((pick_idx % 7) + 1), 'YYYY-MM-DD'),
                    (pick_idx % 7) + 1,
                    estimated,
                    thesis,
                    sentiment,
                    stock_price,
                    target,
                    round((estimated * 0.5)::numeric, 2),
                    round((estimated * 2.0)::numeric, 2),
                    rlevel,
                    cat,
                    mentions,
                    pick_idx,
                    gpt_s,
                    gpt_rat,
                    claude_s,
                    claude_rat,
                    combined
                );

                -- Generate EOD summaries for all days EXCEPT today (the most recent)
                -- so the dashboard shows a "morning mode" picks view for the latest date.
                IF NOT is_today THEN
                    entry_p := estimated;
                    -- Vary win/loss outcome based on a deterministic mix
                    pnl_pct := -0.4 + (((pick_idx * 19 + weekday_count * 23) % 200) / 100.0); -- range -0.4 to +1.6
                    closing_p := round(GREATEST(0.05, entry_p * (1 + pnl_pct))::numeric, 2);
                    stock_open := round(stock_price::numeric, 2);
                    stock_close := round((stock_price * (1 + pnl_pct * 0.05))::numeric, 2);

                    INSERT INTO summaries (
                        date, symbol, contract_type, strike_price, expiration,
                        entry_price, closing_price, stock_open, stock_close, notes
                    ) VALUES (
                        to_char(d, 'YYYY-MM-DD'),
                        sym,
                        ctype,
                        strike,
                        to_char(d + ((pick_idx % 7) + 1), 'YYYY-MM-DD'),
                        entry_p,
                        closing_p,
                        stock_open,
                        stock_close,
                        CASE
                            WHEN closing_p > entry_p * 1.5 THEN 'Strong move on confirmed catalyst, contract gained well above target.'
                            WHEN closing_p > entry_p THEN 'Modest gains as the underlying drifted favorably through the session.'
                            WHEN closing_p < entry_p * 0.6 THEN 'Stock reversed against the thesis after morning gap, contract lost most premium.'
                            ELSE 'Choppy session, contract held value but did not develop a clean trend.'
                        END
                    );
                END IF;
            END LOOP;
        END IF;

        d := d - 1;
    END LOOP;
END $$;

-- Quick sanity check counts (visible in `docker logs vt-local-postgres`)
DO $$
DECLARE
    trade_ct INT;
    summary_ct INT;
    sub_ct INT;
BEGIN
    SELECT COUNT(*) INTO trade_ct FROM trades;
    SELECT COUNT(*) INTO summary_ct FROM summaries;
    SELECT COUNT(*) INTO sub_ct FROM subscribers;
    RAISE NOTICE 'Seed complete: % trades, % summaries, % subscribers', trade_ct, summary_ct, sub_ct;
END $$;
