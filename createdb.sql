CREATE DATABASE db_name;
CREATE USER username WITH PASSWORD 'pa$$w0rd';
\c db_name
CREATE TABLE table_name(
	ID	VARCHAR	PRIMARY KEY	UNIQUE	NOT NULL,
	URL	VARCHAR	UNIQUE	NOT NULL,
	ORIGIN	INET NOT NULL
);
\c db_name
GRANT ALL PRIVILEGES ON DATABASE db_name TO username;
GRANT ALL PRIVILEGES ON TABLE table_name TO username;
