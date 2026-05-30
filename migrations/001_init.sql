-- files: Drive のファイル一覧と同期状態を管理する
CREATE TABLE IF NOT EXISTS files (
  id            BIGSERIAL    PRIMARY KEY,
  drive_file_id TEXT         NOT NULL UNIQUE,
  path          TEXT         NOT NULL,
  checksum      TEXT         NOT NULL,
  mime_type     TEXT         NOT NULL,
  sync_status   TEXT         NOT NULL DEFAULT 'pending',
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- chunks: ファイルを分割したテキスト断片 (将来の AI/RAG 向け)
-- content を embedding に変換して pgvector に保存する想定
CREATE TABLE IF NOT EXISTS chunks (
  id               BIGSERIAL  PRIMARY KEY,
  file_id          BIGINT     NOT NULL REFERENCES files(id),
  chunk_index      INT        NOT NULL,
  content          TEXT       NOT NULL,
  embedding_status TEXT       NOT NULL DEFAULT 'pending'
);

-- jobs: 処理キューとリトライ管理
CREATE TABLE IF NOT EXISTS jobs (
  id          BIGSERIAL    PRIMARY KEY,
  status      TEXT         NOT NULL DEFAULT 'pending',
  retry_count INT          NOT NULL DEFAULT 0,
  started_at  TIMESTAMPTZ
);
