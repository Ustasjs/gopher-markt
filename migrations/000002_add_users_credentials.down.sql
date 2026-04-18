ALTER TABLE users
    DROP COLUMN IF EXISTS login,
    DROP COLUMN IF EXISTS password_hash;
