-- What became of a bar's signal: opened, closed, or why it was not acted on
-- (risk limit, kill switch, closed market, ...). Empty when there was no signal.
-- One or more "kind: message" parts joined by " / ".
ALTER TABLE bar_decisions
    ADD COLUMN result VARCHAR(600) NOT NULL DEFAULT '' AFTER detail;
