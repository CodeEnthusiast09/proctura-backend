-- Modify "exams" table
ALTER TABLE "public"."exams" ADD COLUMN "results_released" boolean NULL DEFAULT false;
