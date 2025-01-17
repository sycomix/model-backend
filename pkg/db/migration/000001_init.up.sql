BEGIN;

CREATE TYPE valid_state AS ENUM (
  'STATE_UNSPECIFIED',
  'STATE_OFFLINE',
  'STATE_ONLINE',
  'STATE_ERROR'
);
CREATE TYPE valid_visibility AS ENUM (
  'VISIBILITY_PUBLIC',
  'VISIBILITY_PRIVATE'
);

CREATE TYPE valid_task AS ENUM (
  'TASK_UNSPECIFIED',
  'TASK_CLASSIFICATION',
  'TASK_DETECTION',
  'TASK_KEYPOINT',
  'TASK_OCR',
  'TASK_INSTANCE_SEGMENTATION',
  'TASK_SEMANTIC_SEGMENTATION',
  'TASK_TEXT_TO_IMAGE',
  'TASK_TEXT_GENERATION'
);

CREATE TYPE valid_release_stage AS ENUM (
  'RELEASE_STAGE_UNSPECIFIED',
  'RELEASE_STAGE_ALPHA',
  'RELEASE_STAGE_BETA',
  'RELEASE_STAGE_GENERALLY_AVAILABLE',
  'RELEASE_STAGE_CUSTOM'
);

CREATE TABLE IF NOT EXISTS "model_definition" (
  "uid" UUID PRIMARY KEY,
  "id" VARCHAR(63) NOT NULL,
  "title" varchar(255) NOT NULL,
  "documentation_url" VARCHAR(1023) NULL,
  "icon" VARCHAR(1023) NULL,
  "release_stage" VALID_RELEASE_STAGE DEFAULT 'RELEASE_STAGE_UNSPECIFIED' NOT NULL,
  "model_spec" JSONB NOT NULL,
  "create_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "update_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "delete_time" timestamptz DEFAULT CURRENT_TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS "model" (
  "uid" UUID PRIMARY KEY,
  "id" VARCHAR(63) NOT NULL,
  "description" varchar(1023) NULL,
  "model_definition_uid" UUID NOT NULL,
  "configuration" JSONB NULL,
  "visibility" VALID_VISIBILITY DEFAULT 'VISIBILITY_PRIVATE' NOT NULL,
  "state" VALID_STATE NOT NULL,
  "task" VALID_TASK NOT NULL,
  "owner" VARCHAR(255) NULL,
  "create_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "update_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "delete_time" timestamptz DEFAULT CURRENT_TIMESTAMP NULL,
  CONSTRAINT fk_model_definition_uid
    FOREIGN KEY ("model_definition_uid")
    REFERENCES model_definition("uid")
);
CREATE UNIQUE INDEX unique_owner_id_delete_time ON model ("owner", "id")
WHERE "delete_time" IS NULL;
CREATE INDEX model_id_create_time_pagination ON model ("id", "create_time");

CREATE TABLE IF NOT EXISTS "triton_model" (
  "uid" UUID PRIMARY KEY,
  "name" varchar(255) NOT NULL,
  "version" int NOT NULL,
  "state" VALID_STATE NOT NULL,
  "platform" varchar(256),
  "create_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "update_time" timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
  "delete_time" timestamptz DEFAULT CURRENT_TIMESTAMP NULL,
  "model_uid" UUID NOT NULL,
  CONSTRAINT fk_triton_model_uid
    FOREIGN KEY ("model_uid")
    REFERENCES model("uid")
    ON DELETE CASCADE
);

COMMIT;
