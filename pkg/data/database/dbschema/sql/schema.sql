-- Version: 1.1
-- Description: Create table urls
CREATE TABLE urls (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    date_created  TIMESTAMP
);