CREATE TABLE load_test_runs (
    id uuid PRIMARY KEY,
    started_at timestamptz NOT NULL,
    payload jsonb NOT NULL
);
CREATE TABLE load_test_results (
    run_id uuid NOT NULL REFERENCES load_test_runs(id),
    sequence integer NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY (run_id, sequence)
);
CREATE INDEX load_test_runs_latest ON load_test_runs(started_at DESC);
CREATE INDEX orders_load_prefix ON orders(client_order_id text_pattern_ops);
