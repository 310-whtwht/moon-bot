-- How a deployment enters a position.
--   entry_order         market: take the quote now (default).
--                       limit:  rest an order at the near side of the quote
--                               (buy at BID, sell at ASK) and wait for a fill.
--   limit_wait_seconds  how long a limit entry may wait before it is cancelled.
--   limit_fallback      what to do after an unfilled limit entry is cancelled:
--                       skip (give the entry up) or market (enter at market).
--   max_spread          entries are skipped while ASK - BID is wider than this
--                       (in the quote currency, e.g. 0.02 JPY). NULL = no limit.
ALTER TABLE deployments
    ADD COLUMN entry_order ENUM('market', 'limit') NOT NULL DEFAULT 'market' AFTER units,
    ADD COLUMN limit_wait_seconds INT NOT NULL DEFAULT 30 AFTER entry_order,
    ADD COLUMN limit_fallback ENUM('skip', 'market') NOT NULL DEFAULT 'skip' AFTER limit_wait_seconds,
    ADD COLUMN max_spread DECIMAL(18, 8) NULL AFTER limit_fallback;
