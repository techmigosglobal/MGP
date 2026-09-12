-- Kept in init for fresh Docker volumes. Existing volumes use db/migrations/002_desktop_workflow.sql.
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS revision_of uuid REFERENCES gate_passes(id) ON DELETE RESTRICT;
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS revision_no integer NOT NULL DEFAULT 0;
ALTER TABLE gate_passes DROP CONSTRAINT IF EXISTS gate_passes_revision_no_check;
ALTER TABLE gate_passes ADD CONSTRAINT gate_passes_revision_no_check CHECK (revision_no >= 0);
CREATE INDEX IF NOT EXISTS gate_passes_revision_idx ON gate_passes(revision_of, revision_no);
