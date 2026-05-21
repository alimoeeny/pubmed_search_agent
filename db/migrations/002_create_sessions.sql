CREATE TABLE pubmed_sessions (
    session_id   TEXT NOT NULL,
    app_name     TEXT NOT NULL,
    user_id      TEXT NOT NULL REFERENCES user_profiles(user_id) ON DELETE CASCADE,
    state        JSONB NOT NULL DEFAULT '{}',
    events       JSONB NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (app_name, user_id, session_id)
);

CREATE INDEX ON pubmed_sessions (user_id, updated_at DESC);
