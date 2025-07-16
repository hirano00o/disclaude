-- Discord Claude システムのデータベーススキーマ

-- ユーザー管理テーブル
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    discord_id VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'user')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- セッション管理テーブル
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    thread_id VARCHAR(255) UNIQUE NOT NULL,
    sandbox_name VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive', 'terminated')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    terminated_at TIMESTAMP WITH TIME ZONE
);

-- サンドボックス管理テーブル
CREATE TABLE IF NOT EXISTS sandboxes (
    id SERIAL PRIMARY KEY,
    session_id INTEGER REFERENCES sessions(id) ON DELETE CASCADE,
    pod_name VARCHAR(255) UNIQUE NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    cpu_limit VARCHAR(50) NOT NULL DEFAULT '1000m',
    memory_limit VARCHAR(50) NOT NULL DEFAULT '2Gi',
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'terminated')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- サンドボックス使用状況テーブル
CREATE TABLE IF NOT EXISTS sandbox_usage (
    id SERIAL PRIMARY KEY,
    current_count INTEGER NOT NULL DEFAULT 0,
    max_count INTEGER NOT NULL DEFAULT 3,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 初期データの挿入
INSERT INTO sandbox_usage (current_count, max_count) VALUES (0, 3)
ON CONFLICT DO NOTHING;

-- インデックスの作成
CREATE INDEX IF NOT EXISTS idx_users_discord_id ON users(discord_id);
CREATE INDEX IF NOT EXISTS idx_sessions_thread_id ON sessions(thread_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sandboxes_session_id ON sandboxes(session_id);
CREATE INDEX IF NOT EXISTS idx_sandboxes_pod_name ON sandboxes(pod_name);
CREATE INDEX IF NOT EXISTS idx_sandboxes_status ON sandboxes(status);

-- 更新日時を自動更新する関数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 更新日時の自動更新トリガー
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sandboxes_updated_at BEFORE UPDATE ON sandboxes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sandbox_usage_updated_at BEFORE UPDATE ON sandbox_usage
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();