# Central Attendance Backend (Go Clean Architecture + PostgreSQL)

Written in Go with **Clean Architecture / Ports & Adapters** and backed by **PostgreSQL**.

## Architecture Structure

```text
server/backend/
├── main.go                     # Composition Root (Dependency Injection)
├── internal/
│   ├── core/
│   │   ├── domain/             # Core Business Entities (POJO / No ORM)
│   │   │   └── models.go
│   │   └── ports/              # Interfaces (Contracts between layers)
│   │       ├── repositories.go
│   │       └── services.go
│   ├── usecase/                # Business Rules & Use Cases
│   │   ├── auth_usecase.go
│   │   ├── user_usecase.go
│   │   ├── attendance_usecase.go
│   │   ├── device_usecase.go
│   │   ├── sync_usecase.go
│   │   ├── enrollment_usecase.go
│   │   └── audit_usecase.go
│   ├── adapters/
│   │   ├── repository/postgres/# PostgreSQL Adapter (pgx/v5)
│   │   │   ├── db.go
│   │   │   ├── admin_device_repo.go
│   │   │   ├── user_repo.go
│   │   │   ├── attendance_repo.go
│   │   │   └── enrollment_audit_repo.go
│   │   └── handler/http/       # HTTP Web API Layer (Gin)
│   │       ├── router.go
│   │       ├── middleware.go
│   │       ├── auth_device_handler.go
│   │       ├── user_handler.go
│   │       ├── attendance_handler.go
│   │       ├── sync_handler.go
│   │       └── enrollment_audit_handler.go
│   └── pkg/
│       ├── hasher/             # Bcrypt & SHA-256 password/token hashing
│       └── token/              # JWT authentication tokens
```

## How to Build and Run

### Configuration (.env)
Copy `.env.example` to `.env`:
```bash
cp .env.example .env
```

### Run directly
```bash
go run main.go
# or
go run .
```

### Build Binary
```bash
go build -o bin/server main.go
./bin/server
```

### Run Tests
```bash
go test ./...
```
