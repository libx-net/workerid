CREATE TABLE workerid_clusters (
    cluster VARCHAR(255) PRIMARY KEY,
    max_worker_id INT NOT NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE TABLE workerid_leases (
    cluster VARCHAR(255) NOT NULL,
    worker_id INT NOT NULL,
    token VARCHAR(64) NULL,
    expire_at TIMESTAMP(6) NOT NULL DEFAULT '1970-01-01 00:00:00.000000',
    PRIMARY KEY (cluster, worker_id)
);

CREATE INDEX workerid_leases_available_idx
ON workerid_leases (cluster, expire_at, worker_id);
