-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS orders
(
    id             SERIAL PRIMARY KEY,
    order_number   VARCHAR(20) UNIQUE,
    user_id        INT,
    contact_name   VARCHAR(100)   NOT NULL,
    contact_phone  VARCHAR(20)    NOT NULL,
    contact_email  VARCHAR(100),
    status_id      INT            NOT NULL,
    total_amount   DECIMAL(10, 2) NOT NULL,
    payment_method VARCHAR(50),
    payment_status VARCHAR(20)             DEFAULT 'pending',
    notes          TEXT,
    ip_address     INET,
    user_agent     TEXT,
    created_at     TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP      NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (status_id) REFERENCES order_statuses (id),
    CONSTRAINT chk_total_amount_positive CHECK (total_amount >= 0)
);

CREATE INDEX idx_orders_order_number ON orders (order_number);
CREATE INDEX idx_orders_contact_phone ON orders (contact_phone);
CREATE INDEX idx_orders_created_at ON orders (created_at);
CREATE INDEX idx_orders_status_id ON orders (status_id);

-- Function to generate order number
CREATE OR REPLACE FUNCTION generate_order_number() RETURNS TRIGGER AS
$$
BEGIN
    NEW.order_number := 'ORD' || to_char(NEW.id, 'FM000000');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to set order number
CREATE TRIGGER set_order_number
    BEFORE INSERT
    ON orders
    FOR EACH ROW
    WHEN (NEW.order_number IS NULL)
EXECUTE FUNCTION generate_order_number();

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_modified_column()
    RETURNS TRIGGER AS
$$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_orders_modtime
    BEFORE UPDATE
    ON orders
    FOR EACH ROW
EXECUTE FUNCTION update_modified_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_orders_modtime ON orders;
DROP TRIGGER IF EXISTS set_order_number ON orders;
DROP FUNCTION IF EXISTS generate_order_number();
DROP FUNCTION IF EXISTS update_modified_column();
DROP TABLE IF EXISTS orders CASCADE;
-- +goose StatementEnd
