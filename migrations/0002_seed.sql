-- Seed data for local development. Nodes are intentionally NOT seeded here:
-- real nodes register themselves via the node-agent gRPC handshake, so seeding
-- them would create phantom nodes that immediately flap to "down" for lack of
-- heartbeats. We only seed team quotas, which the fair-share scheduler and quota
-- checks rely on.

INSERT INTO team_quotas (team_id, max_gpus, max_cpus, max_memory_gb)
VALUES
    ('research', 16, 256, 2048),
    ('inference', 8, 128, 1024),
    ('platform', 4, 64, 512)
ON CONFLICT (team_id) DO NOTHING;
