-- セッション token_hash は毎リクエストで引かれる。B-tree index で O(log n) lookup。
CREATE INDEX IF NOT EXISTS idx_customer_sessions_token_hash ON customer_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_provider_sessions_token_hash ON provider_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_token_hash ON admin_sessions(token_hash);

-- 1 時間ごとの cleanup 時に expires_at < now() でスキャンする。
CREATE INDEX IF NOT EXISTS idx_customer_sessions_expires_at ON customer_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_provider_sessions_expires_at ON provider_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at ON admin_sessions(expires_at);

-- API キー検証は key_hash の完全一致（UNIQUE で自動 index だが明示）。
CREATE INDEX IF NOT EXISTS idx_api_keys_customer_id ON api_keys(customer_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_provided_api_id ON api_keys(provided_api_id);

-- Provider 側 API キー検証の JOIN で使う。
CREATE INDEX IF NOT EXISTS idx_provided_apis_provider_id ON provided_apis(provider_id);

-- Webhook 冪等性チェックで event_id UNIQUE は既に primary key だが、cleanup 用に processed_at。
CREATE INDEX IF NOT EXISTS idx_stripe_events_processed_at ON stripe_events(processed_at);

-- Password reset / verification code lookup。
CREATE INDEX IF NOT EXISTS idx_customer_password_resets_token_hash ON customer_password_resets(token_hash);
CREATE INDEX IF NOT EXISTS idx_customer_verification_codes_customer_id ON customer_verification_codes(customer_id);

-- Stripe customer lookup（webhook から plan 更新時）。
CREATE INDEX IF NOT EXISTS idx_customers_stripe_customer_id ON customers(stripe_customer_id);
