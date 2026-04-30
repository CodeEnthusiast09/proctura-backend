# Proctura Backend

API server for Proctura — a multitenancy online coding exam platform for universities. Students write and submit real code instead of writing on paper.

## Stack

- **Go** + **Gin** (HTTP framework)
- **PostgreSQL** + **GORM** (ORM)
- **Atlas** (database migrations)
- **Judge0** via RapidAPI (code execution)
- **JWT** (authentication)

## Architecture

Row-level multitenancy — every school is a **tenant** identified by subdomain. Tenant is resolved from the `Host` header in production or the `X-Tenant-Subdomain` header for local dev.

### Roles

| Role           | What they can do                                           |
|----------------|------------------------------------------------------------|
| `super_admin`  | Manage schools (tenants)                                   |
| `school_admin` | Manage lecturers and students within their school          |
| `lecturer`     | Create courses, exams, questions, and test cases           |
| `student`      | Take exams, write and submit code                          |

## Getting Started

### Prerequisites

- Go 1.26+
- PostgreSQL
- [Atlas CLI](https://atlasgo.io/docs) (`curl -sSf https://atlasgo.sh | sh`)

### Setup

```bash
# 1. Clone and enter the project
git clone git@github.com:CodeEnthusiast09/proctura-backend.git
cd proctura-backend

# 2. Copy env file and fill in your values
cp .env.example .env

# 3. Create the database
createdb proctura_db

# 4. Run the server (Atlas migrations run automatically on startup)
go run ./cmd/main.go

# Or with make
make run
```

### Environment Variables

| Variable                | Description                              | Example                          |
|-------------------------|------------------------------------------|----------------------------------|
| `PORT`                  | Server port                              | `8080`                           |
| `DATABASE_URL`          | Full Postgres DSN (overrides individual) | `postgres://user:pass@host/db`   |
| `DB_HOST`               | Database host                            | `localhost`                      |
| `DB_PORT`               | Database port                            | `5432`                           |
| `DB_USERNAME`           | Database user                            | `postgres`                       |
| `DB_PASSWORD`           | Database password                        | —                                |
| `DB_DATABASE`           | Database name                            | `proctura_db`                    |
| `JWT_SECRET`            | JWT signing secret                       | —                                |
| `JWT_EXPIRATION`        | Token lifetime                           | `24h`                            |
| `JUDGE0_BASE_URL`       | Judge0 API base URL                      | `https://judge0-ce.p.rapidapi.com` |
| `JUDGE0_API_KEY`        | RapidAPI key                             | —                                |
| `JUDGE0_API_HOST`       | RapidAPI host                            | `judge0-ce.p.rapidapi.com`       |
| `SUPER_ADMIN_EMAIL`     | Super admin email (seeded on boot)       | `admin@proctura.com`             |
| `SUPER_ADMIN_PASSWORD`  | Super admin password (seeded on boot)    | —                                |
| `RESEND_API_KEY`        | Resend API key (primary email provider)  | —                                |
| `EMAIL_FROM`            | Sender address for transactional email   | `Proctura <noreply@proctura.com>`|
| `SMTP_HOST`             | SMTP host (fallback email provider)      | `smtp.gmail.com`                 |
| `SMTP_PORT`             | SMTP port                                | `587`                            |
| `SMTP_USER`             | SMTP username                            | —                                |
| `SMTP_PASSWORD`         | SMTP password                            | —                                |
| `CLOUDINARY_CLOUD_NAME` | Cloudinary cloud name (recordings < 100 MB) | —                             |
| `CLOUDINARY_API_KEY`    | Cloudinary API key                       | —                                |
| `CLOUDINARY_API_SECRET` | Cloudinary API secret                    | —                                |
| `MINIO_ENDPOINT`        | MinIO endpoint (recordings ≥ 100 MB)     | `minio.example.com`              |
| `MINIO_ACCESS_KEY`      | MinIO access key                         | —                                |
| `MINIO_SECRET_KEY`      | MinIO secret key                         | —                                |
| `MINIO_BUCKET`          | MinIO bucket name                        | `proctura`                       |
| `MINIO_USE_SSL`         | Use HTTPS for MinIO                      | `false`                          |
| `MINIO_PUBLIC_URL`      | Public base URL for MinIO objects        | `https://minio.example.com`      |
| `FRONTEND_URL`          | Frontend app URL (used in email links)   | `http://localhost:3000`          |
| `APP_BASE_URL`          | App base URL (CORS allowed origin)       | `http://localhost:8080`          |

## API Reference

All routes are prefixed with `/api/v1`.

### Auth (public)

| Method | Endpoint                  | Description                              |
|--------|---------------------------|------------------------------------------|
| POST   | `/auth/login`             | Login with email + password              |
| POST   | `/auth/register`          | Student self-registration                |
| POST   | `/auth/forgot-password`   | Request password reset token             |
| POST   | `/auth/reset-password`    | Reset password with token                |
| POST   | `/auth/accept-invite`     | Accept lecturer / school admin invite    |

### Super Admin

Requires `Authorization: Bearer <token>` with role `super_admin`.

| Method | Endpoint             | Description          |
|--------|----------------------|----------------------|
| POST   | `/admin/tenants`     | Onboard a new school |
| GET    | `/admin/tenants`     | List all schools     |
| PUT    | `/admin/tenants/:id` | Update a school      |
| DELETE | `/admin/tenants/:id` | Delete a school      |

### School Admin

Requires `Authorization: Bearer <token>` + `X-Tenant-Subdomain: <subdomain>` (local dev).

| Method | Endpoint                       | Description                    |
|--------|--------------------------------|--------------------------------|
| GET    | `/users`                       | List users (filterable by role)|
| POST   | `/users/invite-lecturer`       | Invite a lecturer              |
| POST   | `/users/import-students`       | Bulk import students via CSV   |
| PUT    | `/users/:id`                   | Update user active status      |
| DELETE | `/users/:id`                   | Remove a user                  |

### Lecturer (+ School Admin)

| Method | Endpoint                                      | Description                              |
|--------|-----------------------------------------------|------------------------------------------|
| POST   | `/courses`                                    | Create a course                          |
| PUT    | `/courses/:id`                                | Update a course                          |
| DELETE | `/courses/:id`                                | Delete a course                          |
| POST   | `/courses/:id/enroll`                         | Enroll students by matric number         |
| DELETE | `/courses/:id/enrollments/:studentId`         | Remove a student from a course           |
| GET    | `/courses/:id/enrollments`                    | List enrolled students                   |
| POST   | `/exams`                                      | Create an exam                           |
| PUT    | `/exams/:id`                                  | Update exam (draft only)                 |
| PATCH  | `/exams/:id/status`                           | Update exam status                       |
| DELETE | `/exams/:id`                                  | Delete an exam                           |
| GET    | `/exams/:id/results`                          | View all student submissions for an exam |
| GET    | `/results`                                    | View all results across all exams        |
| GET    | `/submissions/:id`                            | View full submission detail with code    |
| PATCH  | `/submissions/:id/answers/:answerId/score`    | Override score for a specific answer     |
| POST   | `/exams/:id/questions`                        | Add a question                           |
| PUT    | `/questions/:id`                              | Update a question                        |
| DELETE | `/questions/:id`                              | Delete a question                        |
| POST   | `/questions/:id/test-cases`                   | Add test cases                           |
| PUT    | `/test-cases/:id`                             | Update a test case                       |
| DELETE | `/test-cases/:id`                             | Delete a test case                       |

### Shared (all authenticated roles)

| Method | Endpoint       | Description              |
|--------|----------------|--------------------------|
| GET    | `/me`          | Get current user profile |
| GET    | `/courses`     | List courses             |
| GET    | `/exams`       | List exams               |
| GET    | `/exams/:id`   | Get exam details         |

### Student

| Method | Endpoint                          | Description                               |
|--------|-----------------------------------|-------------------------------------------|
| GET    | `/exams/available`                | Get exams open for enrolled students      |
| GET    | `/my-submissions`                 | List all of the student's submissions     |
| GET    | `/exams/:id/my-submission`        | Get student's submission for a given exam |
| POST   | `/exams/:examID/start`            | Start an exam (creates submission)        |
| PUT    | `/submissions/:id/answer`         | Save / update answer for a question       |
| POST   | `/submissions/:id/run`            | Run code against visible test cases       |
| GET    | `/submissions/:id/upload-token`   | Get a provider-routed upload token (`?size=<bytes>`) |
| POST   | `/submissions/:id/submit`         | Final submission (recording attached separately)     |
| PATCH  | `/submissions/:id/recording`      | Attach recording URL after background upload         |
| GET    | `/submissions/:id/result`         | Poll for graded result                    |
| POST   | `/submissions/:id/violation`      | Log anti-cheat violation (tab switch etc) |

## Judge0 Language IDs

| Language | ID |
|----------|----|
| Python 3 | 71 |
| C        | 50 |
| C++      | 54 |
| C#       | 51 |
| Java     | 62 |

## Exam Grading

Grading runs asynchronously after `POST /submissions/:id/submit`:

1. Each answer is sent to Judge0 for evaluation
2. All test cases are run against the submitted code
3. Score per question = `(passed_cases / total_cases) × question.points`
4. Total score is saved when all questions are graded
5. Poll `GET /submissions/:id/result` — status changes from `submitted` → `graded`

## Score Override

Lecturers can manually override the auto-graded score for any answer:

- `PATCH /submissions/:id/answers/:answerId/score` accepts `{ score: int }`
- Score is validated against the question's max points
- `total_score` on the submission is recalculated automatically after each override

## Email

Transactional emails use a **fallback chain** — if the primary provider fails, the next one is tried automatically:

1. **Resend** (primary) — configure `RESEND_API_KEY`
2. **SMTP** (fallback) — configure `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`

If neither is configured, a no-op mailer is used (emails are silently dropped).

### Login Notifications

A security notification email is sent on every successful login:

- Sent asynchronously (does not delay the login response)
- Includes login time, IP address, and geolocation (city/country via ip-api.com)
- Skips geolocation for private/loopback IPs

## Exam Recordings

Webcam recordings are captured in the browser during exams and uploaded directly to cloud storage after submission (background upload — students are not blocked waiting for it).

Storage is routed by file size:

| File size  | Provider   | Notes                              |
|------------|------------|------------------------------------|
| < 100 MB   | Cloudinary | Free tier; direct signed upload    |
| ≥ 100 MB   | MinIO      | Self-hosted S3-compatible storage  |

The frontend fetches a token from `GET /submissions/:id/upload-token?size=<bytes>`, uploads directly to the chosen provider, then PATCHes the submission with the recording URL via `PATCH /submissions/:id/recording`.

> MinIO requires CORS configured on the server to allow `PUT` requests from the frontend origin.

### Local MinIO setup

**1. Start MinIO with Docker**

```bash
docker run -d \
  --name proctura-minio \
  -p 9000:9000 \
  -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -e MINIO_API_CORS_ALLOW_ORIGIN="http://localhost:3000" \
  -v ~/minio-data:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

**2. Install the MinIO client and create the bucket**

```bash
yay -S minio-client          # binary is named mcli on Arch
mcli alias set local http://localhost:9000 minioadmin minioadmin
mcli mb local/proctura
```

**3. Add to `.env`**

```env
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=proctura
MINIO_USE_SSL=false
MINIO_PUBLIC_URL=http://localhost:9000
```

The MinIO web console is available at `http://localhost:9001` (login: `minioadmin` / `minioadmin`).

> **Note:** Newer MinIO versions (post-2023) handle CORS via the `MINIO_API_CORS_ALLOW_ORIGIN` environment variable — the `mcli cors set` command is no longer supported.

## Anti-Cheat

- Tab switching, window blur, fullscreen exit, and clipboard events are detected on the frontend
- Each event calls `POST /submissions/:id/violation`
- After **3 violations**, the submission is automatically submitted

## Commands

```bash
make run                                        # start the server
make build                                      # compile to ./bin/proctura
make test                                       # run all tests (requires proctura_test_db)

make migrate-diff name=add_phone_to_users       # generate a new migration from model changes
make migrate-hash                               # rehash atlas.sum after editing migrations
make migrate-apply url=<DSN>                    # apply pending migrations manually
make migrate-status url=<DSN>                   # show applied vs pending migrations
make migrate-validate                           # verify atlas.sum integrity
```

---

## Database Migrations (Atlas)

This project uses [Atlas](https://atlasgo.io) with the GORM provider to manage database migrations. Understanding the flow is important — there are two distinct concerns: **generating** migrations and **applying** them.

### How it works

Atlas reads your GORM model definitions and compares them against the current migration directory to generate SQL diff files. The app then applies those SQL files automatically at startup via the embedded Atlas Go library (`database.RunMigrations`).

```
GORM models  →  atlas migrate diff  →  .sql files  →  app startup applies them
```

`atlas.hcl` is only needed for **generating** migrations — not for running the app.

### Prerequisites

Install the Atlas CLI:

```bash
curl -sSf https://atlasgo.sh | sh
```

Also ensure `atlas-provider-gorm` is available (it's already in `go.mod`).

### Workflow: adding a new migration

**1. Make your model changes**

Edit or add structs in `internal/models/`. GORM tags drive the schema.

**2. Generate the migration**

```bash
atlas migrate diff <descriptive_name> --env gorm
```

Atlas reads your GORM models, compares them against the last migration state, and writes a new `.sql` file to `migrations/`.

Example:
```bash
atlas migrate diff add_courses_table --env gorm
# creates: migrations/20250416143200_add_courses_table.sql
```

**3. Rehash the migration directory**

```bash
atlas migrate hash --env gorm
```

This updates `migrations/atlas.sum` — the integrity file Atlas uses to detect tampering. Run this every time you add or edit a migration file. `atlas.sum` is auto-generated; always commit it alongside the `.sql` file.

**4. Apply migrations**

You rarely need to do this manually — migrations run automatically when the server starts. But you can apply manually against any database:

```bash
atlas migrate apply --env gorm --url "postgresql://user:pass@localhost:5432/proctura_db?sslmode=disable"
```

### Key files

| File | Purpose |
|------|---------|
| `atlas.hcl` | Config: tells Atlas where your models are, where to write migrations, and which `dev` DB to use |
| `migrations/*.sql` | The actual SQL migration files (committed to version control) |
| `migrations/atlas.sum` | Integrity checksum — auto-generated by `atlas migrate hash`, always commit it |

### The `dev` database

Atlas needs a scratch database to compute diffs — it creates, drops, and modifies schemas freely inside it. This **must not be your real database**. The project is configured to use `proctura_dev_db`:

```bash
createdb proctura_dev_db   # create once, never touch it manually
```

### Inspecting migration status

```bash
# See which migrations are pending
atlas migrate status --env gorm --url "postgresql://user:pass@localhost:5432/proctura_db?sslmode=disable"

# Validate the migration directory integrity
atlas migrate validate --env gorm
```

---

## Running Tests

Requires a `proctura_test_db` database:

```bash
createdb proctura_test_db

DB_USERNAME=<user> DB_PASSWORD=<pass> DB_DATABASE=proctura_test_db \
  go test ./... -v -p 1
```

> `-p 1` runs packages sequentially to avoid concurrent AutoMigrate races on the shared test database.

## Docker

```bash
docker build -t proctura-backend .
docker run -p 8080:8080 --env-file .env proctura-backend
```
