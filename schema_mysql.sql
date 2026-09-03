-- Greenfield MySQL 5.7 / 8.x schema. The library does not auto-DDL.
--
-- CREATE TABLE IF NOT EXISTS does not alter an existing workerid_leases table.
-- If expire_at is still TIMESTAMP(6) (applied under a relaxed sql_mode), run:
--
--   ALTER TABLE workerid_leases
--     MODIFY expire_at DATETIME(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000';
--
-- If workerid_leases_available_idx is missing:
--
--   ALTER TABLE workerid_leases
--     ADD INDEX workerid_leases_available_idx (cluster, expire_at, worker_id);

CREATE TABLE IF NOT EXISTS workerid_clusters (
    cluster VARCHAR(255) PRIMARY KEY,
    max_worker_id INT NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE TABLE IF NOT EXISTS workerid_leases (
    cluster VARCHAR(255) NOT NULL,
    worker_id INT NOT NULL,
    token VARCHAR(64) NULL,
    expire_at DATETIME(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000',
    PRIMARY KEY (cluster, worker_id),
    INDEX workerid_leases_available_idx (cluster, expire_at, worker_id)
);
