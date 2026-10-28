CREATE TABLE pangu.catalog_bindings (
 namespace text NOT NULL,
 project_id uuid NOT NULL REFERENCES pangu.projects(id),
 key uuid NOT NULL,
 request jsonb NOT NULL,
 object_id uuid REFERENCES pangu.catalog_objects(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(namespace, project_id, key)
);
