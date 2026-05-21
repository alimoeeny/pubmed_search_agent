CREATE TYPE subscription_plan AS ENUM ('free', 'pro', 'max');

CREATE TABLE user_profiles (
    user_id       TEXT PRIMARY KEY,          -- matches Supabase auth.users.id (JWT sub)
    email         TEXT NOT NULL,
    plan          subscription_plan NOT NULL DEFAULT 'free',
    enabled       BOOLEAN NOT NULL DEFAULT true,
    model_default TEXT,                       -- optional per-user model override
    pdf_style     TEXT,                       -- optional per-user PDF style prompt
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
