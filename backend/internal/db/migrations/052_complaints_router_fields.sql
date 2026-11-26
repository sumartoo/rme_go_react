-- Align complaints with the n8n smart-router body {title, subject, message, type}:
-- keep the title and subject sent to n8n, and make the customer name optional.
ALTER TABLE complaints
    ADD COLUMN IF NOT EXISTS title   varchar(100) NOT NULL DEFAULT 'AI Assistant',
    ADD COLUMN IF NOT EXISTS subject varchar(150) NOT NULL DEFAULT 'Analyze Customer Complaint',
    ALTER COLUMN customer_name SET DEFAULT '';
