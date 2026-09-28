-- shapes.sql — every required kind, a table's columns (nesting), a
-- multi-line callable signature, and FK references (plain + qualified).
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    total NUMERIC,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE VIEW active_users AS
    SELECT * FROM users WHERE active = TRUE;

CREATE UNIQUE INDEX idx_users_email ON users(email);

CREATE FUNCTION calculate_total(
    p_order_id INT,
    p_discount NUMERIC
)
RETURNS NUMERIC AS $$
BEGIN
    RETURN 0;
END;
$$ LANGUAGE plpgsql;

CREATE PROCEDURE process_order(IN p_id INT)
LANGUAGE plpgsql AS $$
BEGIN
    -- no-op
END;
$$;

CREATE TYPE order_status AS ENUM ('pending', 'shipped', 'delivered');
