-- Drop redundant individual indexes now covered by the composite unique index
DROP INDEX "public"."idx_course_enrollments_course_id";
DROP INDEX "public"."idx_course_enrollments_student_id";
