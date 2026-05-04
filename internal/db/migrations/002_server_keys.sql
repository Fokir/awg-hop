ALTER TABLE ingress_settings ADD COLUMN server_private_key TEXT NOT NULL DEFAULT '';
ALTER TABLE ingress_settings ADD COLUMN server_public_key TEXT NOT NULL DEFAULT '';
