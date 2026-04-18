-- Create "course_enrollments" table
CREATE TABLE "public"."course_enrollments" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "tenant_id" uuid NOT NULL,
  "course_id" uuid NOT NULL,
  "student_id" uuid NOT NULL,
  "created_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_course_enrollments_course" FOREIGN KEY ("course_id") REFERENCES "public"."courses" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "fk_course_enrollments_student" FOREIGN KEY ("student_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_course_enrollments_course_id" to table: "course_enrollments"
CREATE INDEX "idx_course_enrollments_course_id" ON "public"."course_enrollments" ("course_id");
-- Create index "idx_course_enrollments_student_id" to table: "course_enrollments"
CREATE INDEX "idx_course_enrollments_student_id" ON "public"."course_enrollments" ("student_id");
-- Create index "idx_course_enrollments_tenant_id" to table: "course_enrollments"
CREATE INDEX "idx_course_enrollments_tenant_id" ON "public"."course_enrollments" ("tenant_id");
-- Create unique index to prevent duplicate enrollments
CREATE UNIQUE INDEX "idx_enrollment_course_student" ON "public"."course_enrollments" ("course_id", "student_id");
