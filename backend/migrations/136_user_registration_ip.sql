-- 用户注册来源 IP 与归属地（管理端用户列表展示）。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS register_ip_address VARCHAR(45) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS register_ip_country VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS register_ip_country_code VARCHAR(16) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS register_ip_region VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS register_ip_city VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS register_ip_location VARCHAR(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_register_ip_address
    ON users(register_ip_address)
    WHERE register_ip_address <> '';

COMMENT ON COLUMN users.register_ip_address IS '注册时记录的客户端 IP';
COMMENT ON COLUMN users.register_ip_country IS '注册 IP 归属国家/地区';
COMMENT ON COLUMN users.register_ip_country_code IS '注册 IP 国家/地区代码';
COMMENT ON COLUMN users.register_ip_region IS '注册 IP 归属省/州/区域';
COMMENT ON COLUMN users.register_ip_city IS '注册 IP 归属城市';
COMMENT ON COLUMN users.register_ip_location IS '注册 IP 归属地展示文本';
