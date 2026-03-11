CREATE DATABASE IF NOT EXISTS instagram_auth         CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS instagram_users        CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS instagram_social       CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS instagram_posts        CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS instagram_interactions CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

GRANT ALL PRIVILEGES ON instagram_auth.*         TO 'spale'@'%';
GRANT ALL PRIVILEGES ON instagram_users.*        TO 'spale'@'%';
GRANT ALL PRIVILEGES ON instagram_social.*       TO 'spale'@'%';
GRANT ALL PRIVILEGES ON instagram_posts.*        TO 'spale'@'%';
GRANT ALL PRIVILEGES ON instagram_interactions.* TO 'spale'@'%';
FLUSH PRIVILEGES;
