-- quoted.sql — schema-qualified/quoted names, OR REPLACE, MATERIALIZED,
-- TEMPORARY and IF NOT EXISTS variants (the incumbent's own coverage).
-- Grammar variance: this grammar's identifier text keeps surrounding quote
-- characters as written (unlike the incumbent's stripQuotes) — see
-- PROVENANCE.md.
CREATE TABLE IF NOT EXISTS "public"."orders" (
    id SERIAL PRIMARY KEY
);

CREATE TABLE `schema_b`.`products` (
    id SERIAL PRIMARY KEY
);

CREATE TEMPORARY TABLE temp_staging (
    row_data TEXT
);

CREATE OR REPLACE VIEW active_users AS
    SELECT * FROM users;

CREATE MATERIALIZED VIEW IF NOT EXISTS order_summary AS
    SELECT user_id FROM orders;

CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id);
