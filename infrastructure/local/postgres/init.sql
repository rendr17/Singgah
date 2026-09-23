-- Disposable database for integration tests (TEST_DATABASE_URL).
-- Extensions are created by migrations, not here — singgah_test goes through
-- the same goose path as the main database.
CREATE DATABASE singgah_test;
