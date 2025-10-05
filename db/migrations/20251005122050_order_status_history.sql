-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_status_history (
    id SERIAL PRIMARY KEY,
    order_id INT NOT NULL,
    status_id INT NOT NULL,
    changed_by INT,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    FOREIGN KEY (status_id) REFERENCES order_statuses(id)
);

-- Indexes
CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
CREATE INDEX idx_order_status_history_created_at ON order_status_history(created_at);

-- Trigger function to log status changes
CREATE OR REPLACE FUNCTION log_order_status_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.status_id IS DISTINCT FROM NEW.status_id THEN
        INSERT INTO order_status_history (order_id, status_id, changed_by, notes)
        VALUES (NEW.id, NEW.status_id, NULL, 'Status changed from ' || 
               (SELECT name FROM order_statuses WHERE id = OLD.status_id) || ' to ' ||
               (SELECT name FROM order_statuses WHERE id = NEW.status_id));
    ELSIF TG_OP = 'INSERT' THEN
        INSERT INTO order_status_history (order_id, status_id, changed_by, notes)
        VALUES (NEW.id, NEW.status_id, NULL, 'Order created with status: ' || 
               (SELECT name FROM order_statuses WHERE id = NEW.status_id));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for order status changes
CREATE TRIGGER order_status_change_trigger
AFTER INSERT OR UPDATE OF status_id ON orders
FOR EACH ROW
EXECUTE FUNCTION log_order_status_change();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS order_status_change_trigger ON orders;
DROP FUNCTION IF EXISTS log_order_status_change();
DROP TABLE IF EXISTS order_status_history CASCADE;
-- +goose StatementEnd
