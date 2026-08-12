CREATE TABLE IF NOT EXISTS users (
    id       SERIAL PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email    TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS href (
    id       SERIAL PRIMARY KEY,
    url      TEXT NOT NULL UNIQUE,
    long_url TEXT
);

CREATE TABLE IF NOT EXISTS userhref (
    id      SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    href_id INTEGER NOT NULL REFERENCES href (id) ON DELETE CASCADE,
    UNIQUE (user_id, href_id)
);

CREATE TABLE IF NOT EXISTS click (
    id      SERIAL PRIMARY KEY,
    href_id INTEGER NOT NULL REFERENCES href (id) ON DELETE CASCADE,
    ip      TEXT NOT NULL,
    country TEXT NOT NULL,
    time    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_userhref_user_id ON userhref (user_id);
CREATE INDEX IF NOT EXISTS idx_click_href_id ON click (href_id);
