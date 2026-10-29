package postgres

import (
	"context"
	"database/sql"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type userRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) ports.UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*entity.User, error) {
	query := `
	SELECT u.id, u.employee_id, u.title, u.first_name, u.last_name, u.name, u.department, u.status, u.version, u.deleted_at, u.created_at, u.updated_at,
	       (SELECT COUNT(*) FROM user_embeddings e WHERE e.user_id = u.id) > 0 as has_embedding
	FROM users u
	WHERE u.id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanUser(row)
}

func (r *userRepo) GetByEmployeeID(ctx context.Context, employeeID string) (*entity.User, error) {
	query := `
	SELECT u.id, u.employee_id, u.title, u.first_name, u.last_name, u.name, u.department, u.status, u.version, u.deleted_at, u.created_at, u.updated_at,
	       (SELECT COUNT(*) FROM user_embeddings e WHERE e.user_id = u.id) > 0 as has_embedding
	FROM users u
	WHERE u.employee_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, employeeID)
	return scanUser(row)
}

func (r *userRepo) List(ctx context.Context, unenrolledOnly bool) ([]*entity.User, error) {
	query := `
	SELECT u.id, u.employee_id, u.title, u.first_name, u.last_name, u.name, u.department, u.status, u.version, u.deleted_at, u.created_at, u.updated_at,
	       (SELECT COUNT(*) FROM user_embeddings e WHERE e.user_id = u.id) > 0 as has_embedding
	FROM users u
	WHERE u.deleted_at IS NULL
	`
	if unenrolledOnly {
		query += " AND (SELECT COUNT(*) FROM user_embeddings e WHERE e.user_id = u.id) = 0"
	}
	query += " ORDER BY u.created_at DESC"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*entity.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *userRepo) Create(ctx context.Context, u *entity.User) error {
	query := `
	INSERT INTO users (id, employee_id, title, first_name, last_name, name, department, status, version, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT COALESCE(MAX(version), 0) + 1 FROM users), $9, $10)
	RETURNING version
	`
	err := r.db.QueryRowContext(ctx, query,
		u.ID, u.EmployeeID, u.Title, u.FirstName, u.LastName, u.Name, u.Department, u.Status, u.CreatedAt, u.UpdatedAt).Scan(&u.Version)
	return err
}

func (r *userRepo) Update(ctx context.Context, u *entity.User) error {
	query := `
	UPDATE users SET
		employee_id = $1, title = $2, first_name = $3, last_name = $4, name = $5, department = $6, status = $7,
		version = (SELECT COALESCE(MAX(version), 0) + 1 FROM users),
		updated_at = $8
	WHERE id = $9
	RETURNING version
	`
	err := r.db.QueryRowContext(ctx, query,
		u.EmployeeID, u.Title, u.FirstName, u.LastName, u.Name, u.Department, u.Status, u.UpdatedAt, u.ID).Scan(&u.Version)
	return err
}

func (r *userRepo) Delete(ctx context.Context, id string) error {
	query := `
	UPDATE users SET
		deleted_at = CURRENT_TIMESTAMP,
		status = 'inactive',
		version = (SELECT COALESCE(MAX(version), 0) + 1 FROM users)
	WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *userRepo) GetLatestVersion(ctx context.Context) (int, error) {
	var maxV sql.NullInt64
	err := r.db.QueryRowContext(ctx, "SELECT MAX(version) FROM users").Scan(&maxV)
	if err != nil || !maxV.Valid {
		return 0, nil
	}
	return int(maxV.Int64), nil
}

func (r *userRepo) ListUpdatedSince(ctx context.Context, sinceVersion int) ([]*entity.User, error) {
	query := `
	SELECT u.id, u.employee_id, u.title, u.first_name, u.last_name, u.name, u.department, u.status, u.version, u.deleted_at, u.created_at, u.updated_at,
	       (e.id IS NOT NULL) as has_embedding,
	       e.embedding
	FROM users u
	LEFT JOIN user_embeddings e ON u.id = e.user_id
	WHERE u.version > $1
	ORDER BY u.version ASC
	`
	rows, err := r.db.QueryContext(ctx, query, sinceVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*entity.User
	for rows.Next() {
		var u entity.User
		var empID, title, fName, lName, dept sql.NullString
		var deletedAt sql.NullTime
		var rawEmb []byte

		if err := rows.Scan(
			&u.ID, &empID, &title, &fName, &lName, &u.Name, &dept, &u.Status, &u.Version, &deletedAt, &u.CreatedAt, &u.UpdatedAt, &u.HasEmbedding, &rawEmb,
		); err != nil {
			return nil, err
		}

		u.EmployeeID = empID.String
		u.Title = title.String
		u.FirstName = fName.String
		u.LastName = lName.String
		u.Department = dept.String
		if deletedAt.Valid {
			u.DeletedAt = &deletedAt.Time
		}
		u.RawEmbedding = rawEmb

		users = append(users, &u)
	}
	return users, nil
}

func (r *userRepo) SaveEmbedding(ctx context.Context, emb *entity.UserEmbedding) error {
	query := `
	INSERT INTO user_embeddings (id, user_id, embedding, model_version, source, device_id, quality_score, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (user_id) DO UPDATE SET
		id = EXCLUDED.id,
		embedding = EXCLUDED.embedding,
		model_version = EXCLUDED.model_version,
		source = EXCLUDED.source,
		device_id = EXCLUDED.device_id,
		quality_score = EXCLUDED.quality_score,
		created_at = EXCLUDED.created_at
	`
	_, err := r.db.ExecContext(ctx, query,
		emb.ID, emb.UserID, emb.Embedding, emb.ModelVersion, emb.Source, emb.DeviceID, emb.QualityScore, emb.CreatedAt)
	return err
}

func (r *userRepo) GetEmbedding(ctx context.Context, userID string) (*entity.UserEmbedding, error) {
	query := `
	SELECT id, user_id, embedding, model_version, source, device_id, quality_score, created_at
	FROM user_embeddings WHERE user_id = $1
	ORDER BY created_at DESC LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, userID)
	var e entity.UserEmbedding
	var deviceID sql.NullString
	if err := row.Scan(&e.ID, &e.UserID, &e.Embedding, &e.ModelVersion, &e.Source, &deviceID, &e.QualityScore, &e.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	e.DeviceID = deviceID.String
	return &e, nil
}

func (r *userRepo) DeleteEmbedding(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM user_embeddings WHERE user_id = $1", userID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner rowScanner) (*entity.User, error) {
	var u entity.User
	var empID, title, fName, lName, dept sql.NullString
	var deletedAt sql.NullTime

	if err := scanner.Scan(
		&u.ID, &empID, &title, &fName, &lName, &u.Name, &dept, &u.Status, &u.Version, &deletedAt, &u.CreatedAt, &u.UpdatedAt, &u.HasEmbedding,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}

	u.EmployeeID = empID.String
	u.Title = title.String
	u.FirstName = fName.String
	u.LastName = lName.String
	u.Department = dept.String
	if deletedAt.Valid {
		u.DeletedAt = &deletedAt.Time
	}
	return &u, nil
}
