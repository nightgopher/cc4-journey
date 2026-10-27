CREATE TABLE reimbursement_claims (
    id serial PRIMARY KEY,
    version integer NOT NULL DEFAULT 1,
    employee_id integer NOT NULL REFERENCES employees(id),
    title varchar(160) NOT NULL DEFAULT '票据报销',
    category varchar(32) NOT NULL DEFAULT 'other' CHECK(category IN ('office','transport','travel','meals','other')),
    merchant varchar(200) NOT NULL DEFAULT '',
    invoice_number varchar(64) NOT NULL DEFAULT '',
    invoice_code varchar(64) NOT NULL DEFAULT '',
    invoice_key text,
    invoice_date date,
    currency varchar(8) NOT NULL DEFAULT 'CNY',
    invoice_total numeric(12,2) CHECK(invoice_total >= 0),
    amount numeric(12,2) CHECK(amount >= 0),
    reason varchar(2000) NOT NULL DEFAULT '',
    human_edited boolean NOT NULL DEFAULT false,
    review_note varchar(2000) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','submitted','approved','rejected','paid','cancelled')),
    receipt_file varchar(80) NOT NULL,
    receipt_mime varchar(80) NOT NULL,
    receipt_hash varchar(64) NOT NULL,
    receipt_pages integer NOT NULL DEFAULT 1,
    ocr_status varchar(20) NOT NULL DEFAULT 'queued' CHECK(ocr_status IN ('queued','submitting','running','ready','failed')),
    ocr_model varchar(40) NOT NULL,
    ocr_job_id varchar(128),
    ocr_generation integer NOT NULL DEFAULT 1,
    ocr_lease varchar(32),
    ocr_locked_until timestamptz,
    ocr_started_at timestamptz,
    ocr_next_poll timestamptz NOT NULL DEFAULT now(),
    ocr_attempts integer NOT NULL DEFAULT 0,
    ocr_progress jsonb NOT NULL DEFAULT '{}',
    ocr_result jsonb,
    ocr_error varchar(500),
    confirmed_at timestamptz,
    submitted_at timestamptz,
    approved_by integer REFERENCES employees(id),
    approved_at timestamptz,
    paid_by integer REFERENCES employees(id),
    paid_at timestamptz,
    payment_date date,
    finance_record_id integer UNIQUE REFERENCES finance_records(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK(status IN ('draft','rejected','cancelled') OR
          (amount > 0 AND amount <= invoice_total AND invoice_date IS NOT NULL AND currency='CNY' AND confirmed_at IS NOT NULL)),
    CHECK(status != 'paid' OR (finance_record_id IS NOT NULL AND paid_by IS NOT NULL)),
    CHECK(approved_by IS NULL OR approved_by != employee_id),
    CHECK(paid_by IS NULL OR paid_by != employee_id)
);
CREATE UNIQUE INDEX reimbursement_receipt_once ON reimbursement_claims(receipt_hash) WHERE status!='cancelled';
CREATE UNIQUE INDEX reimbursement_invoice_once ON reimbursement_claims(invoice_key) WHERE invoice_key IS NOT NULL AND status!='cancelled';
CREATE INDEX reimbursement_owner_status ON reimbursement_claims(employee_id,status,id DESC);
CREATE INDEX reimbursement_ocr_due ON reimbursement_claims(ocr_next_poll) WHERE ocr_status IN ('queued','submitting','running');
CREATE UNIQUE INDEX reimbursement_finance_once ON finance_records(reference_id) WHERE reference_type='reimbursement';
CREATE TABLE reimbursement_events (
    id serial PRIMARY KEY,
    claim_id integer NOT NULL REFERENCES reimbursement_claims(id),
    actor_id integer REFERENCES employees(id),
    action varchar(32) NOT NULL,
    note varchar(2000) NOT NULL DEFAULT '',
    details jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
