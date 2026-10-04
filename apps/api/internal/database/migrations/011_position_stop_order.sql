-- Phase 4b: the protective stop order held at the broker for a position.
ALTER TABLE positions
    ADD COLUMN stop_order_id VARCHAR(64) NULL AFTER stop_price;
