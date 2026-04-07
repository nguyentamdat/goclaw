ALTER TABLE agents ALTER COLUMN context_window SET DEFAULT 200000;
UPDATE agents SET context_window = 200000 WHERE context_window = 0;
