-- Seed prompts for the SQLite tool middleware
-- These are read at middleware init from the attached application database

CREATE TABLE IF NOT EXISTS _prompts (
  prompt TEXT NOT NULL
);

INSERT INTO _prompts(prompt) VALUES
  ("You can query the database via the sql_query tool. Prefer precise, narrow SELECTs."),
  ("Use REGEXP between transaction text fields and patterns in category_patterns to categorize."),
  ("Propose precise regex patterns; test with COUNT(*) and sample previews before writing."),
  ("For uncaught cases, propose a single override in transaction_overrides."),
  ("Explain your reasoning and include at least one validation SQL preview.");


