-- Complete the server-backed form contract used by the reference control room.
ALTER TABLE users ADD COLUMN IF NOT EXISTS rank text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS signature_path text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS signature_sha256 text NOT NULL DEFAULT '';

ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS loaded_in_presence_of text NOT NULL DEFAULT '';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS carrier_name text NOT NULL DEFAULT '';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS carrier_designation text NOT NULL DEFAULT '';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS remarks text NOT NULL DEFAULT '';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS copy_type text NOT NULL DEFAULT 'ORIGINAL';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS approval_reference text NOT NULL DEFAULT '';
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS record_hash text NOT NULL DEFAULT '';

ALTER TABLE gate_pass_items ADD COLUMN IF NOT EXISTS category text NOT NULL DEFAULT '';
ALTER TABLE gate_pass_items ADD COLUMN IF NOT EXISTS serial_no text NOT NULL DEFAULT '';
ALTER TABLE gate_pass_items ADD COLUMN IF NOT EXISTS batch_no text NOT NULL DEFAULT '';
ALTER TABLE gate_pass_items ADD COLUMN IF NOT EXISTS full_part text NOT NULL DEFAULT 'Full Item';

ALTER TABLE gate_passes DROP CONSTRAINT IF EXISTS gate_passes_copy_type_check;
ALTER TABLE gate_passes ADD CONSTRAINT gate_passes_copy_type_check CHECK (copy_type IN ('ORIGINAL', 'DUPLICATE', 'TRIPLICATE'));
