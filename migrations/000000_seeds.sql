BEGIN;

INSERT INTO printers (name, model, is_active, nickname, location, serial_number)
VALUES
    ('warehouse-printer-01', 'label-printer-model-a', TRUE, 'Packing station', 'Warehouse A', 'SN-123'),
    ('warehouse-printer-02', 'label-printer-model-a', TRUE, NULL, NULL, NULL),
    ('office-printer-01', 'label-printer-model-b', FALSE, 'Old office printer', 'Office', 'SN-456')
ON CONFLICT (name) DO NOTHING;

INSERT INTO labels (id, name, version, is_active, nickname, is_primary)
VALUES
    ('label-123', 'Shipping label', '3', TRUE, 'Standard shipping', TRUE),
    ('label-456', 'Return label', '1', TRUE, NULL, FALSE),
    ('label-789', 'Legacy shipping label', '2', FALSE, 'Legacy', FALSE)
ON CONFLICT (id) DO NOTHING;

UPDATE inventory_sync_state
SET last_successful_sync_at = CURRENT_TIMESTAMP
WHERE last_successful_sync_at IS NULL;

COMMIT;
