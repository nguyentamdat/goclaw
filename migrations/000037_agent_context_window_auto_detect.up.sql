-- Allow model registry to auto-detect context_window.
-- Change default from 200000 to 0; resolver falls back to DefaultContextWindow when registry has no data.
ALTER TABLE agents ALTER COLUMN context_window SET DEFAULT 0;
UPDATE agents SET context_window = 0 WHERE context_window = 200000;
