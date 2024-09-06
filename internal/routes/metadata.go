package routes

import (
	"database/sql"
	"fsd/pkg/tasks"
	"net/http"
	"time"

	"github.com/go-chi/render"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type MetadataController struct{}

type Metadata struct {
	ID          int64     `json:"id"`
	FullPath    string    `json:"full_path"`
	SizeBytes   int64     `json:"size_bytes"`
	FileMode    int64     `json:"file_mode"`
	IsDirectory int       `json:"is_directory"`
	CreatedAt   time.Time `json:"created_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

func RowsToMetadata(rows *sql.Rows) ([]Metadata, error) {
	var metas []Metadata
	for rows.Next() {
		var meta Metadata
		if err := rows.Scan(
			&meta.ID,
			&meta.FullPath,
			&meta.SizeBytes,
			&meta.FileMode,
			&meta.IsDirectory,
			&meta.CreatedAt,
			&meta.ModifiedAt,
		); err != nil {
			return nil, err
		}

		metas = append(metas, meta)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return metas, nil
}

func (m *MetadataController) GetMetadata(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", tasks.FSD_DB_FILENAME)
	if err != nil {
		zap.L().Error("failed to open metadata database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := `SELECT * FROM metadata ORDER BY created_at DESC`
	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	metadata, err := RowsToMetadata(rows)
	if err != nil {
		zap.L().Error("failed to parse metadata", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}

	if len(metadata) == 0 {
		zap.L().Warn("no metadata found")
		render.Status(r, http.StatusNotFound)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, metadata)
}

func (m *MetadataController) GetLatestMetadata(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", tasks.FSD_DB_FILENAME)
	if err != nil {
		zap.L().Error("failed to open metadata database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Get the most recent metadata for each unique full_path
	query := `
		SELECT m.*
		FROM metadata m
		INNER JOIN (
			SELECT full_path, MAX(created_at) as max_created_at
			FROM metadata
			GROUP BY full_path
		) latest
		ON m.full_path = latest.full_path AND m.created_at = latest.max_created_at
		ORDER BY m.full_path
	`

	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	metadata, err := RowsToMetadata(rows)
	if err != nil {
		zap.L().Error("failed to parse metadata", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}

	if len(metadata) == 0 {
		zap.L().Warn("no metadata found")
		render.Status(r, http.StatusNotFound)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, metadata)
}
