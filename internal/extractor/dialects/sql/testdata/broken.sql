-- broken.sql — one syntax error the parser must survive: this grammar has
-- no create_procedure production at all (PROVENANCE.md), so the whole
-- CREATE PROCEDURE statement below becomes an ERROR node — and the table
-- before it and the view after it must still extract cleanly.
CREATE TABLE survivor_before (
    id SERIAL PRIMARY KEY
);

CREATE PROCEDURE broken_proc(IN p_id INT)
LANGUAGE plpgsql AS $$
BEGIN
    NULL;
END;
$$;

CREATE VIEW survivor_after AS
    SELECT * FROM survivor_before;
