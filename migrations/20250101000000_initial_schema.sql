-- Create tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    subdomain  TEXT NOT NULL UNIQUE,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID REFERENCES tenants(id) ON DELETE CASCADE,
    email               TEXT NOT NULL UNIQUE,
    password_hash       TEXT NOT NULL,
    role                VARCHAR(20) NOT NULL CHECK (role IN ('super_admin', 'school_admin', 'lecturer', 'student')),
    first_name          TEXT NOT NULL,
    last_name           TEXT NOT NULL,
    matric_number       TEXT,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    is_verified         BOOLEAN NOT NULL DEFAULT FALSE,
    invite_token        TEXT,
    reset_token         TEXT,
    reset_token_expiry  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_tenant_id    ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_matric_number ON users(matric_number);
CREATE INDEX IF NOT EXISTS idx_users_invite_token  ON users(invite_token);
CREATE INDEX IF NOT EXISTS idx_users_reset_token   ON users(reset_token);

-- Create courses table
CREATE TABLE IF NOT EXISTS courses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    lecturer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    code        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_courses_tenant_id   ON courses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_courses_lecturer_id ON courses(lecturer_id);

-- Create exams table
CREATE TABLE IF NOT EXISTS exams (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    course_id        UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    instructions     TEXT,
    duration_minutes INT NOT NULL,
    starts_at        TIMESTAMPTZ NOT NULL,
    ends_at          TIMESTAMPTZ NOT NULL,
    language_id      INT NOT NULL,
    language_name    TEXT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'active', 'closed')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exams_tenant_id ON exams(tenant_id);
CREATE INDEX IF NOT EXISTS idx_exams_course_id ON exams(course_id);

-- Create questions table
CREATE TABLE IF NOT EXISTS questions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id     UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    body        TEXT NOT NULL,
    order_index INT NOT NULL DEFAULT 0,
    points      INT NOT NULL DEFAULT 10,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_questions_exam_id ON questions(exam_id);

-- Create test_cases table
CREATE TABLE IF NOT EXISTS test_cases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id     UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    input           TEXT,
    expected_output TEXT NOT NULL,
    is_hidden       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_test_cases_question_id ON test_cases(question_id);

-- Create submissions table
CREATE TABLE IF NOT EXISTS submissions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    exam_id         UUID NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'submitted', 'graded')),
    started_at      TIMESTAMPTZ NOT NULL,
    submitted_at    TIMESTAMPTZ,
    total_score     INT NOT NULL DEFAULT 0,
    max_score       INT NOT NULL DEFAULT 0,
    violation_count INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submissions_tenant_id  ON submissions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_submissions_exam_id    ON submissions(exam_id);
CREATE INDEX IF NOT EXISTS idx_submissions_student_id ON submissions(student_id);

-- Create submission_answers table
CREATE TABLE IF NOT EXISTS submission_answers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id  UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    question_id    UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    code           TEXT,
    score          INT NOT NULL DEFAULT 0,
    judge0_token   TEXT,
    stdout         TEXT,
    stderr         TEXT,
    compile_output TEXT,
    status_desc    TEXT,
    test_results   TEXT[],
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submission_answers_submission_id ON submission_answers(submission_id);
