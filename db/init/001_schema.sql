CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$ BEGIN CREATE TYPE mgp_role AS ENUM ('ADMIN', 'INVENTORY', 'ISSUING', 'SECURITY', 'VIEWER'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE gate_pass_status AS ENUM ('DRAFT', 'SUBMITTED', 'APPROVED', 'NOT_APPROVED', 'PASSED_OUT', 'RETURNED'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE gate_pass_type AS ENUM ('RETURNABLE', 'NON_RETURNABLE'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS system_settings (
  key text PRIMARY KEY,
  value jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid
);

CREATE TABLE IF NOT EXISTS inventory_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  item_code text NOT NULL,
  item_name text NOT NULL,
  category text NOT NULL DEFAULT '',
  serial_no text NOT NULL DEFAULT '',
  batch_no text NOT NULL DEFAULT '',
  unit_of_measure text NOT NULL DEFAULT 'NOS',
  quantity numeric(18,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
  holder text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  archived_at timestamptz,
  archived_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS inventory_items_active_identity_key ON inventory_items (item_code, serial_no, batch_no) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS consignees (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  address text NOT NULL DEFAULT '',
  contact text NOT NULL DEFAULT '',
  archived_at timestamptz,
  archived_by uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS consignees_active_name_key ON consignees (lower(name)) WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS pass_sequences (
  directorate text NOT NULL,
  project text NOT NULL,
  year integer NOT NULL CHECK (year BETWEEN 2000 AND 9999),
  current_number integer NOT NULL DEFAULT 0 CHECK (current_number >= 0),
  PRIMARY KEY (directorate, project, year)
);

CREATE TABLE IF NOT EXISTS gate_passes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  pass_no text NOT NULL UNIQUE,
  pass_type gate_pass_type NOT NULL,
  status gate_pass_status NOT NULL DEFAULT 'DRAFT',
  revision_of uuid REFERENCES gate_passes(id) ON DELETE RESTRICT,
  revision_no integer NOT NULL DEFAULT 0 CHECK (revision_no >= 0),
  pass_date date NOT NULL,
  directorate text NOT NULL,
  project text NOT NULL,
  inventory_no text NOT NULL DEFAULT '',
  inventory_holder text NOT NULL DEFAULT '',
  consignee_id uuid REFERENCES consignees(id),
  consignee_name text NOT NULL,
  consignee_address text NOT NULL DEFAULT '',
  reference_no text NOT NULL DEFAULT '',
  packages integer NOT NULL CHECK (packages > 0),
  purpose text NOT NULL,
  authority text NOT NULL,
  vehicle_no text NOT NULL DEFAULT '',
  loaded_in_presence_of text NOT NULL DEFAULT '',
  carrier_name text NOT NULL DEFAULT '',
  carrier_designation text NOT NULL DEFAULT '',
  expected_return_date date,
  actual_return_date date,
  security_control_no text,
  rejection_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid NOT NULL,
  submitted_at timestamptz,
  approved_at timestamptz,
  approved_by uuid,
  passed_out_at timestamptz,
  security_officer_id uuid,
  returned_at timestamptz,
  returned_by uuid,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid NOT NULL,
  CHECK ((pass_type = 'RETURNABLE' AND expected_return_date IS NOT NULL) OR pass_type = 'NON_RETURNABLE'),
  CHECK (actual_return_date IS NULL OR pass_type = 'RETURNABLE')
);
CREATE INDEX IF NOT EXISTS gate_passes_status_idx ON gate_passes(status, pass_date DESC);
CREATE INDEX IF NOT EXISTS gate_passes_creator_idx ON gate_passes(created_by, status);
CREATE INDEX IF NOT EXISTS gate_passes_revision_idx ON gate_passes(revision_of, revision_no);

CREATE TABLE IF NOT EXISTS gate_pass_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  gate_pass_id uuid NOT NULL REFERENCES gate_passes(id) ON DELETE RESTRICT,
  inventory_item_id uuid REFERENCES inventory_items(id) ON DELETE SET NULL,
  item_code text NOT NULL DEFAULT '',
  item_name text NOT NULL,
  category text NOT NULL DEFAULT '',
  serial_no text NOT NULL DEFAULT '',
  batch_no text NOT NULL DEFAULT '',
  unit_of_measure text NOT NULL DEFAULT 'NOS',
  full_or_part text NOT NULL DEFAULT 'FULL',
  quantity numeric(18,3) NOT NULL CHECK (quantity > 0),
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS gate_pass_items_pass_idx ON gate_pass_items(gate_pass_id);

CREATE TABLE IF NOT EXISTS gate_pass_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  gate_pass_id uuid REFERENCES gate_passes(id) ON DELETE RESTRICT,
  action text NOT NULL,
  from_status gate_pass_status,
  to_status gate_pass_status,
  actor_id uuid NOT NULL,
  actor_role mgp_role NOT NULL,
  reason text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id uuid NOT NULL DEFAULT gen_random_uuid(),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS gate_pass_events_pass_idx ON gate_pass_events(gate_pass_id, created_at);

CREATE OR REPLACE FUNCTION prevent_gate_pass_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'gate_pass_events are append-only';
END;
$$;
DROP TRIGGER IF EXISTS gate_pass_events_immutable ON gate_pass_events;
CREATE TRIGGER gate_pass_events_immutable BEFORE UPDATE OR DELETE ON gate_pass_events FOR EACH ROW EXECUTE FUNCTION prevent_gate_pass_event_mutation();

CREATE OR REPLACE FUNCTION next_gate_pass_number(p_directorate text, p_project text, p_year integer)
RETURNS text LANGUAGE plpgsql AS $$
DECLARE next_number integer;
BEGIN
  INSERT INTO pass_sequences(directorate, project, year, current_number)
  VALUES (p_directorate, p_project, p_year, 1)
  ON CONFLICT (directorate, project, year)
  DO UPDATE SET current_number = pass_sequences.current_number + 1
  RETURNING current_number INTO next_number;
  RETURN p_directorate || '/' || p_project || '/' || p_year::text || '/' || lpad(next_number::text, 4, '0');
END;
$$;

INSERT INTO system_settings(key, value) VALUES
  ('organization', '{"name":"","address":"","defaultDirectorate":"","defaultProject":""}'::jsonb),
  ('backup', '{"lastSuccessfulAt":null}'::jsonb)
ON CONFLICT (key) DO NOTHING;
