-- Sample SQL for syntax highlighting

-- Create database
CREATE DATABASE IF NOT EXISTS sample_db;
USE sample_db;

-- Create tables
CREATE TABLE users (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    status ENUM('active', 'inactive', 'pending') DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_email (email),
    INDEX idx_status (status)
);

CREATE TABLE posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT,
    published BOOLEAN DEFAULT FALSE,
    view_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE tags (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE post_tags (
    post_id BIGINT NOT NULL,
    tag_id INT NOT NULL,
    PRIMARY KEY (post_id, tag_id),
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Insert sample data
INSERT INTO users (username, email, password_hash, first_name, last_name, status)
VALUES
    ('alice', 'alice@example.com', 'hash123', 'Alice', 'Smith', 'active'),
    ('bob', 'bob@example.com', 'hash456', 'Bob', 'Jones', 'active'),
    ('charlie', 'charlie@example.com', 'hash789', 'Charlie', 'Brown', 'pending');

INSERT INTO posts (user_id, title, content, published)
VALUES
    (1, 'Introduction to SQL', 'SQL is a powerful language...', TRUE),
    (1, 'Advanced Queries', 'Let us explore joins and subqueries...', TRUE),
    (2, 'Database Design', 'Good design principles...', FALSE);

INSERT INTO tags (name) VALUES ('sql'), ('database'), ('tutorial');

-- Select queries
SELECT * FROM users WHERE status = 'active';

SELECT
    u.username,
    u.email,
    COUNT(p.id) AS post_count
FROM users u
LEFT JOIN posts p ON u.id = p.user_id
GROUP BY u.id, u.username, u.email
HAVING post_count > 0
ORDER BY post_count DESC
LIMIT 10;

-- Subquery
SELECT *
FROM posts
WHERE user_id IN (
    SELECT id FROM users WHERE status = 'active'
);

-- Common Table Expression (CTE)
WITH active_users AS (
    SELECT id, username, email
    FROM users
    WHERE status = 'active'
),
user_posts AS (
    SELECT user_id, COUNT(*) as cnt
    FROM posts
    WHERE published = TRUE
    GROUP BY user_id
)
SELECT
    au.username,
    COALESCE(up.cnt, 0) as published_posts
FROM active_users au
LEFT JOIN user_posts up ON au.id = up.user_id;

-- Update
UPDATE users
SET status = 'active', updated_at = NOW()
WHERE status = 'pending'
    AND created_at < DATE_SUB(NOW(), INTERVAL 7 DAY);

-- Delete
DELETE FROM posts
WHERE published = FALSE
    AND created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);

-- Aggregate functions
SELECT
    status,
    COUNT(*) as user_count,
    MIN(created_at) as first_user,
    MAX(created_at) as last_user
FROM users
GROUP BY status;

-- Window functions
SELECT
    username,
    email,
    created_at,
    ROW_NUMBER() OVER (ORDER BY created_at) as row_num,
    RANK() OVER (PARTITION BY status ORDER BY created_at) as status_rank
FROM users;

-- Case expression
SELECT
    username,
    CASE status
        WHEN 'active' THEN 'Active User'
        WHEN 'inactive' THEN 'Inactive User'
        ELSE 'Pending Verification'
    END AS status_label
FROM users;

-- Create view
CREATE OR REPLACE VIEW active_user_posts AS
SELECT
    u.username,
    p.title,
    p.created_at
FROM users u
INNER JOIN posts p ON u.id = p.user_id
WHERE u.status = 'active' AND p.published = TRUE;

-- Stored procedure
DELIMITER //
CREATE PROCEDURE GetUserPosts(IN user_id BIGINT)
BEGIN
    SELECT p.*, u.username
    FROM posts p
    JOIN users u ON p.user_id = u.id
    WHERE p.user_id = user_id
    ORDER BY p.created_at DESC;
END //
DELIMITER ;

-- Transaction
START TRANSACTION;
    UPDATE users SET status = 'inactive' WHERE id = 1;
    DELETE FROM posts WHERE user_id = 1 AND published = FALSE;
COMMIT;

-- ROLLBACK; -- Uncomment to rollback instead
