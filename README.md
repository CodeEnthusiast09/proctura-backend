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
git clone <repo-url>
cd proctura-backend

# 2. Copy env file and fill in your values
cp .env.example .env

# 3. Create the database
createdb proctura_db

# 4. Run the server (Atlas migrations run automatically on startup)
go run ./cmd/main.go
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
| `APP_BASE_URL`          | App base URL                             | `http://localhost:8080`          |

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

| Method | Endpoint                          | Description                    |
|--------|-----------------------------------|--------------------------------|
| POST   | `/courses`                        | Create a course                |
| PUT    | `/courses/:id`                    | Update a course                |
| DELETE | `/courses/:id`                    | Delete a course                |
| POST   | `/exams`                          | Create an exam                 |
| PUT    | `/exams/:id`                      | Update exam (draft only)       |
| DELETE | `/exams/:id`                      | Delete an exam                 |
| GET    | `/exams/:id/results`              | View all student submissions   |
| POST   | `/exams/:id/questions`            | Add a question                 |
| PUT    | `/questions/:id`                  | Update a question              |
| DELETE | `/questions/:id`                  | Delete a question              |
| POST   | `/questions/:id/test-cases`       | Add a test case                |
| PUT    | `/test-cases/:id`                 | Update a test case             |
| DELETE | `/test-cases/:id`                 | Delete a test case             |

### Shared (all authenticated roles)

| Method | Endpoint       | Description              |
|--------|----------------|--------------------------|
| GET    | `/me`          | Get current user profile |
| GET    | `/courses`     | List courses             |
| GET    | `/exams`       | List exams               |
| GET    | `/exams/:id`   | Get exam details         |

### Student

| Method | Endpoint                          | Description                              |
|--------|-----------------------------------|------------------------------------------|
| GET    | `/exams/available`                | Get exams currently open for submission  |
| POST   | `/exams/:examID/start`            | Start an exam (creates submission)       |
| PUT    | `/submissions/:id/answer`         | Save / update answer for a question      |
| POST   | `/submissions/:id/submit`         | Final submission (triggers grading)      |
| GET    | `/submissions/:id/result`         | Get graded result                        |
| POST   | `/submissions/:id/violation`      | Log anti-cheat violation (tab switch etc)|

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

## Anti-Cheat

- Tab switching and clipboard events are detected on the frontend
- Each event calls `POST /submissions/:id/violation`
- After **3 violations**, the submission is automatically submitted

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
