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
    risk_level TEXT NOT NULL DEFAULT '',
    catalyst TEXT NOT NULL DEFAULT '',
    mention_count INTEGER NOT NULL DEFAULT 0,
    rank INTEGER NOT NULL DEFAULT 0,
    gpt_score INTEGER NOT NULL DEFAULT 0,
    gpt_rationale TEXT NOT NULL DEFAULT '',
    claude_score INTEGER NOT NULL DEFAULT 0,
    claude_rationale TEXT NOT NULL DEFAULT '',
    combined_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    picked_by_openai BOOLEAN NOT NULL DEFAULT false,
    picked_by_claude BOOLEAN NOT NULL DEFAULT false,
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
-- ~1 year of trade history (252 weekdays) with 14 union picks each
-- (4 OpenAI-only + 6 consensus + 4 Claude-only). Mix of winners and
-- losers across CALL/PUT, varied risk levels, varied sentiment. The
-- most recent date is fully populated WITHOUT summaries (simulating a
-- "morning picks" pre-EOD state). All other days have full EOD
-- summaries so /history and /models render rich cumulative-P&L curves.

DO $$
DECLARE
    -- 252 trading days (~1 year), ending today, skipping weekends.
    -- We compute these in SQL so the seed always reflects "the last 252 weekdays"
    -- relative to whenever the docker stack first boots.
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
        'Bullish flag forming on the daily chart. Market signals show accelerating mention count on StockTwits with positive bias.',
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
        'Multiple confirming signals: StockTwits trending score is up 3x, technicals are clean, and IV is reasonable for the move we expect.',
        'Decent risk/reward but not a top-tier setup. Including in the list because the broader sector tailwind supports the thesis.',
        'Speculative tail-risk play. Cheap premium and the upside scenario is asymmetric — comfortable losing the entire premium here.',
        'Mean-reversion candidate. The stock is oversold on RSI and the option premium prices in more downside than I think will materialize.',
        'Earnings volatility play. The expected move priced in is smaller than the historical move on similar setups for this name.',
        'Sentiment-driven momentum. StockTwits is loud on this name and the contract benefits from short-dated gamma into the catalyst.',
        'Hedge against broader exposure. Negative beta to SPY makes this useful as a portfolio protection layer for the day.',
        'Lowest-ranked pick. Including for diversification but conviction is the weakest of the day; would skip if forced to take fewer.'
    ];
    claude_rationales TEXT[] := ARRAY[
        'Confirmed: option chain pricing and underlying technical setup match GPT thesis. Greeks favor the direction with theta risk well managed.',
        'Agree on direction but flag IV percentile at 88th — there is real vega risk if the catalyst lands soft. Score reflects that downside.',
        'Strong technical case is real, but I cannot independently verify the StockTwits trending velocity claim without the raw data. Discounting slightly.',
        'Moderate conviction. Sector tailwind argument is sound but the specific name has weaker relative strength than peers cited.',
        'Cheap premium claim verified, but the asymmetric upside is conditioned on a specific catalyst date I could not confirm via search.',
        'RSI oversold confirmed. However, mean reversion in trending markets is a frequent trap — would rate higher if there were a clearer reversal signal.',
        'Earnings setup is real and the implied move does look low vs the realized history. High-confidence agreement with the analyzer.',
        'Sentiment momentum is observable but the specific contract greeks are mediocre — gamma exposure is fine, theta drag is steeper than implied.',
        'Hedge thesis is sound. Negative correlation to SPY is well established for this name on the timeframe.',
        'Marginal pick. Liquidity is thin enough that even a small fill could move the mark; would not take this in size.'
    ];
BEGIN
    -- Walk back from today, picking weekdays. For each day we generate
    -- 14 unique union picks: 4 only OpenAI picked, 6 BOTH picked
    -- (consensus), 4 only Claude picked. picked_by_openai / picked_by_claude
    -- flags are set accordingly so the All / OpenAI / Claude filter in
    -- the nav bar visibly slices the data.
    d := today_date;
    WHILE weekday_count < 252 LOOP
        IF EXTRACT(DOW FROM d) NOT IN (0, 6) THEN
            weekday_count := weekday_count + 1;
            is_today := (weekday_count = 1);

            FOR pick_idx IN 1..14 LOOP
                -- Deterministic but varied selection. Different
                -- multipliers per pick_idx slot guarantee 14 distinct
                -- tickers per day from a 30-symbol pool.
                sym := symbols[1 + ((pick_idx * 11 + weekday_count * 5) % array_length(symbols, 1))];
                ctype := contract_types[1 + ((pick_idx + weekday_count) % 10)];
                rlevel := risk_levels[1 + ((pick_idx + weekday_count * 2) % 10)];
                cat := catalysts[1 + ((pick_idx + weekday_count) % 10)];
                thesis := theses[1 + (((pick_idx - 1) % 10))];

                -- Pseudo-random but stable price levels.
                stock_price := 50 + ((ascii(substr(sym, 1, 1)) * 7 + pick_idx * 13) % 350);
                strike := round((stock_price + (CASE WHEN ctype = 'CALL' THEN 5 ELSE -5 END) + ((pick_idx * 3) % 15) - 7)::numeric, 0);
                estimated := round((0.50 + ((pick_idx * 17 + weekday_count * 5) % 175) / 100.0)::numeric, 2);
                target := round((stock_price + (CASE WHEN ctype = 'CALL' THEN 8 ELSE -8 END))::numeric, 2);
                sentiment := round((-0.5 + ((pick_idx * 11 + weekday_count * 7) % 150) / 100.0)::numeric, 2);
                mentions := 50 + ((pick_idx * 23 + weekday_count * 11) % 800);

                -- Picker attribution:
                --   pick_idx 1..4   → only OpenAI (claude_score = 0)
                --   pick_idx 5..10  → both (consensus picks, both real scores)
                --   pick_idx 11..14 → only Claude (gpt_score = 0)
                IF pick_idx <= 10 THEN
                    gpt_s := GREATEST(1, LEAST(10, (11 - pick_idx) + (((pick_idx * 13 + weekday_count * 7 + ascii(substr(sym, 1, 1))) % 7) - 3)));
                ELSE
                    gpt_s := 0;
                END IF;

                IF pick_idx >= 5 THEN
                    claude_s := GREATEST(1, LEAST(10, (15 - pick_idx) + (((pick_idx * 17 + weekday_count * 11 + ascii(substr(sym, 1, 1)) * 3) % 7) - 3)));
                ELSE
                    claude_s := 0;
                END IF;

                -- Combined score is the average of the non-zero model scores.
                IF gpt_s > 0 AND claude_s > 0 THEN
                    combined := (gpt_s + claude_s) / 2.0;
                ELSIF gpt_s > 0 THEN
                    combined := gpt_s;
                ELSE
                    combined := claude_s;
                END IF;

                gpt_rat := CASE WHEN gpt_s > 0 THEN gpt_rationales[1 + ((pick_idx - 1) % 10)] ELSE '' END;
                claude_rat := CASE WHEN claude_s > 0 THEN claude_rationales[1 + ((pick_idx - 1) % 10)] ELSE '' END;

                INSERT INTO trades (
                    date, symbol, contract_type, strike_price, expiration, dte,
                    estimated_price, thesis, sentiment_score, current_price,
                    target_price, stop_loss, risk_level,
                    catalyst, mention_count, rank,
                    gpt_score, gpt_rationale, claude_score, claude_rationale, combined_score,
                    picked_by_openai, picked_by_claude
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
                    rlevel,
                    cat,
                    mentions,
                    0, -- rank placeholder, computed below
                    gpt_s,
                    gpt_rat,
                    claude_s,
                    claude_rat,
                    combined,
                    (gpt_s > 0),
                    (claude_s > 0)
                );

                -- Generate EOD summaries for all days EXCEPT today (the most recent)
                -- so the dashboard shows a "morning mode" picks view for the latest date.
                IF NOT is_today THEN
                    entry_p := estimated;
                    -- Realistic P&L distribution skewed to roughly 50/50
                    -- winners and losers, with a long right tail. Some
                    -- trades wipe out completely (closing floor 0.05),
                    -- some double, some flatline. This produces visible
                    -- negative cumulative-P&L stretches on the equity
                    -- curve and a meaningful spread between models when
                    -- they pick different trades.
                    pnl_pct := -1.0 + (((pick_idx * 23 + weekday_count * 31 + ascii(substr(sym, 1, 1))) % 280) / 100.0); -- range -1.00 to +1.80
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

            -- Assign ranks for this day: consensus picks first (both models),
            -- then single-model picks, sorted by combined_score desc within
            -- each tier. This matches the production unionPicks() sort.
            WITH ranked AS (
                SELECT id,
                       ROW_NUMBER() OVER (
                           ORDER BY
                               (picked_by_openai AND picked_by_claude) DESC,
                               combined_score DESC,
                               symbol
                       ) AS new_rank
                FROM trades
                WHERE date = to_char(d, 'YYYY-MM-DD')
            )
            UPDATE trades t
            SET rank = r.new_rank
            FROM ranked r
            WHERE t.id = r.id;
        END IF;

        d := d - 1;
    END LOOP;
END $$;

-- ─── Auto-execution demo data ──────────────────────────────────────────────
--
-- Seeds the execution_decisions + executions tables with 5 demo positions
-- spanning the different badge states, so the local dashboard / history /
-- trade detail pages have visible examples of the transparency labeling
-- before any real auto-execution has fired in prod.
--
-- Mode mix: 4 paper + 1 live (so you can see both color treatments).
-- State mix: holding / closed-winner / closed-loser / failed / closed-live.
--
-- Each execution targets the rank-1 trade for the chosen day so the badge
-- renders on the most prominent card.

-- New columns the runtime migration would add anyway (ADD COLUMN IF NOT
-- EXISTS) — declared here so the seed schema isn't stale.
ALTER TABLE trades ADD COLUMN IF NOT EXISTS gpt_rank INTEGER NOT NULL DEFAULT 0;
ALTER TABLE trades ADD COLUMN IF NOT EXISTS claude_rank INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS execution_decisions (
    id              SERIAL PRIMARY KEY,
    trade_date      TEXT NOT NULL,
    symbol          TEXT NOT NULL,
    contract_type   TEXT NOT NULL,
    strike_price    DOUBLE PRECISION NOT NULL,
    expiration      TEXT NOT NULL,
    occ_symbol      TEXT NOT NULL,
    contract_price  DOUBLE PRECISION NOT NULL,
    gpt_score       INTEGER NOT NULL,
    claude_score    INTEGER NOT NULL,
    trade_id        INTEGER REFERENCES trades(id),
    token_hash      TEXT NOT NULL UNIQUE,
    decision        TEXT NOT NULL DEFAULT 'pending',
    decided_at      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(trade_date)
);

CREATE TABLE IF NOT EXISTS executions (
    id                  SERIAL PRIMARY KEY,
    decision_id         INTEGER NOT NULL REFERENCES execution_decisions(id),
    mode                TEXT NOT NULL,
    side                TEXT NOT NULL,
    schwab_order_id     TEXT,
    status              TEXT NOT NULL,
    fill_price          DOUBLE PRECISION,
    filled_quantity     INTEGER NOT NULL DEFAULT 0,
    requested_quantity  INTEGER NOT NULL,
    submitted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    filled_at           TIMESTAMPTZ,
    error_message       TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Helper: insert a complete execution scenario (decision + open + maybe
-- close) targeting the rank-1 trade on a given date. Returns silently if
-- no trade exists for that date (e.g. weekend skip).
DO $$
DECLARE
    today_date DATE := CURRENT_DATE;
    scenarios RECORD;
    t RECORD;
    decision_id INT;
    fill_open NUMERIC;
    fill_close NUMERIC;
    occ TEXT;
BEGIN
    FOR scenarios IN
        -- (offset_days, mode, state, fill_open_pct, fill_close_pct)
        --   offset_days: how many days back from today
        --   state: 'holding' | 'closed-win' | 'closed-loss' | 'failed'
        --   fill_open_pct: open fill as % of trade.estimated_price (1.0 = exactly mark)
        --   fill_close_pct: close fill as % of fill_open (1.0 = breakeven, 1.5 = +50% etc.)
        SELECT * FROM (VALUES
            (0,  'paper', 'holding',    1.00, NULL),       -- TODAY: paper position still open
            (2,  'paper', 'closed-win', 1.00, 1.45),       -- 2 days ago: paper closed +45%
            (4,  'paper', 'closed-loss',1.00, 0.62),       -- 4 days ago: paper closed -38%
            (6,  'live',  'closed-win', 0.98, 1.80),       -- 6 days ago: LIVE position +80%
            (8,  'paper', 'failed',     NULL, NULL)        -- 8 days ago: open rejected by broker
        ) AS s(offset_days, mode, state, fill_open_pct, fill_close_pct)
    LOOP
        -- Find the rank-1 trade for the target date.
        SELECT id, symbol, contract_type, strike_price, expiration, estimated_price, gpt_score, claude_score
        INTO t
        FROM trades
        WHERE date = to_char(today_date - scenarios.offset_days, 'YYYY-MM-DD')
          AND rank = 1
        LIMIT 1;

        CONTINUE WHEN t IS NULL;

        -- Build a valid OCC-21 OSI symbol for the contract.
        occ := rpad(t.symbol, 6, ' ')
            || to_char(to_date(t.expiration, 'YYYY-MM-DD'), 'YYMMDD')
            || CASE WHEN t.contract_type = 'CALL' THEN 'C' ELSE 'P' END
            || lpad((round(t.strike_price * 1000))::text, 8, '0');

        INSERT INTO execution_decisions (
            trade_date, symbol, contract_type, strike_price, expiration,
            occ_symbol, contract_price, gpt_score, claude_score, trade_id,
            token_hash, decision, decided_at, expires_at
        ) VALUES (
            to_char(today_date - scenarios.offset_days, 'YYYY-MM-DD'),
            t.symbol, t.contract_type, t.strike_price, t.expiration,
            occ, t.estimated_price,
            GREATEST(t.gpt_score, 9), GREATEST(t.claude_score, 9), -- demo data: pretend both ranked it 9+
            t.id,
            md5(scenarios.offset_days::text || scenarios.state || 'demo-token-hash'),
            'execute',
            (today_date - scenarios.offset_days)::timestamptz + interval '9 hours 31 minutes',
            (today_date - scenarios.offset_days)::timestamptz + interval '9 hours 35 minutes'
        )
        ON CONFLICT (trade_date) DO NOTHING
        RETURNING id INTO decision_id;

        -- ON CONFLICT may have skipped — re-fetch if so.
        IF decision_id IS NULL THEN
            SELECT id INTO decision_id FROM execution_decisions
            WHERE trade_date = to_char(today_date - scenarios.offset_days, 'YYYY-MM-DD');
        END IF;

        -- Open execution row.
        IF scenarios.state = 'failed' THEN
            INSERT INTO executions (decision_id, mode, side, status, requested_quantity, error_message, submitted_at)
            VALUES (
                decision_id, scenarios.mode, 'open', 'failed', 1,
                'Demo: simulated broker rejection (insufficient buying power)',
                (today_date - scenarios.offset_days)::timestamptz + interval '9 hours 31 minutes 12 seconds'
            );
        ELSE
            fill_open := round((t.estimated_price * scenarios.fill_open_pct)::numeric, 2);
            INSERT INTO executions (decision_id, mode, side, schwab_order_id, status, fill_price, filled_quantity, requested_quantity, submitted_at, filled_at)
            VALUES (
                decision_id, scenarios.mode, 'open',
                CASE WHEN scenarios.mode = 'paper' THEN 'paper-' || md5(decision_id::text || 'open') ELSE '10000' || decision_id::text END,
                'filled', fill_open, 1, 1,
                (today_date - scenarios.offset_days)::timestamptz + interval '9 hours 31 minutes 12 seconds',
                (today_date - scenarios.offset_days)::timestamptz + interval '9 hours 31 minutes 18 seconds'
            );

            -- Close execution row (only for closed states).
            IF scenarios.state IN ('closed-win', 'closed-loss') THEN
                fill_close := round((fill_open * scenarios.fill_close_pct)::numeric, 2);
                INSERT INTO executions (decision_id, mode, side, schwab_order_id, status, fill_price, filled_quantity, requested_quantity, submitted_at, filled_at)
                VALUES (
                    decision_id, scenarios.mode, 'close',
                    CASE WHEN scenarios.mode = 'paper' THEN 'paper-' || md5(decision_id::text || 'close') ELSE '20000' || decision_id::text END,
                    'filled', fill_close, 1, 1,
                    (today_date - scenarios.offset_days)::timestamptz + interval '15 hours 55 minutes',
                    (today_date - scenarios.offset_days)::timestamptz + interval '15 hours 55 minutes 8 seconds'
                );
            END IF;
        END IF;
    END LOOP;
END $$;

-- Quick sanity check counts (visible in `docker logs vt-local-postgres`)
DO $$
DECLARE
    trade_ct INT;
    summary_ct INT;
    sub_ct INT;
    decision_ct INT;
    exec_ct INT;
BEGIN
    SELECT COUNT(*) INTO trade_ct FROM trades;
    SELECT COUNT(*) INTO summary_ct FROM summaries;
    SELECT COUNT(*) INTO sub_ct FROM subscribers;
    SELECT COUNT(*) INTO decision_ct FROM execution_decisions;
    SELECT COUNT(*) INTO exec_ct FROM executions;
    RAISE NOTICE 'Seed complete: % trades, % summaries, % subscribers, % execution_decisions, % executions',
        trade_ct, summary_ct, sub_ct, decision_ct, exec_ct;
END $$;
