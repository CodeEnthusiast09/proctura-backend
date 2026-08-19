-- Modify "submissions" table
ALTER TABLE "public"."submissions" ADD COLUMN "auto_submitted" boolean NOT NULL DEFAULT false;
