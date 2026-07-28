CREATE TABLE workerid_clusters (
    cluster TEXT PRIMARY KEY,
    max_worker_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workerid_leases (
    cluster TEXT NOT NULL,
    worker_id INTEGER NOT NULL,
    token TEXT,
    expire_at TIMESTAMPTZ NOT NULL DEFAULT TIMESTAMPTZ 'epoch',
    PRIMARY KEY (cluster, worker_id)
);

CREATE INDEX workerid_leases_available_idx
ON workerid_leases (cluster, expire_at, worker_id);
