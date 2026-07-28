CREATE TABLE workerid_clusters (
    cluster TEXT PRIMARY KEY,
    max_worker_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE workerid_leases (
    cluster TEXT NOT NULL,
    worker_id INTEGER NOT NULL,
    token TEXT,
    expire_at TEXT NOT NULL DEFAULT '1970-01-01 00:00:00',
    PRIMARY KEY (cluster, worker_id)
);

CREATE INDEX workerid_leases_available_idx
ON workerid_leases (cluster, expire_at, worker_id);
