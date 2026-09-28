-- A partitioned table and the ALTER TABLE shapes around it (D2 re-measurement).
CREATE TABLE payment (
    payment_id integer NOT NULL
);
ALTER TABLE ONLY payment ATTACH PARTITION payment_p1 FOR VALUES FROM (1) TO (2);
ALTER TABLE payment ADD COLUMN amount numeric;
