CREATE TABLE IF NOT EXISTS sales (
    id SERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    total_items INTEGER NOT NULL DEFAULT 0,
    sold_items INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    sale_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    image_url VARCHAR(500),
    is_sold BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE CASCADE
);


CREATE TABLE IF NOT EXISTS checkout_attempts (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    item_id BIGINT NOT NULL,
    sale_id BIGINT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE,
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS purchases (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    item_id BIGINT NOT NULL,
    sale_id BIGINT NOT NULL,
    checkout_code VARCHAR(255) NOT NULL,
    purchased_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (item_id) REFERENCES items(id) ON DELETE CASCADE,
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_sale_limits (
    user_id VARCHAR(255) NOT NULL,
    sale_id BIGINT NOT NULL,
    items_purchased INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, sale_id),
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_items_sale_id ON items(sale_id);
CREATE INDEX IF NOT EXISTS idx_items_is_sold ON items(is_sold);
CREATE INDEX IF NOT EXISTS idx_purchases_user_id ON purchases(user_id);
CREATE INDEX IF NOT EXISTS idx_checkout_attempts_expires_at ON checkout_attempts(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sale_limits_user_sale ON user_sale_limits(user_id, sale_id);

CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'set_sales_timestamp') THEN
        CREATE TRIGGER set_sales_timestamp
        BEFORE UPDATE ON sales
        FOR EACH ROW
        EXECUTE FUNCTION trigger_set_timestamp();
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'set_items_timestamp') THEN
        CREATE TRIGGER set_items_timestamp
        BEFORE UPDATE ON items
        FOR EACH ROW
        EXECUTE FUNCTION trigger_set_timestamp();
    END IF;
END
$$;