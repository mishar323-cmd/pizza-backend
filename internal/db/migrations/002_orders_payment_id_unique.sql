-- Make payment_id (YooKassa payment.id) a deduplication key so both the
-- client-side success-page POST and the YooKassa webhook can persist the same
-- order without creating duplicates.
CREATE UNIQUE INDEX IF NOT EXISTS orders_payment_id_uniq
  ON orders(payment_id)
  WHERE payment_id IS NOT NULL AND payment_id <> '';
