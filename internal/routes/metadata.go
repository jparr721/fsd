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
	ID          string    `json:"id"`
	FullPath    string    `json:"full_path"`
	SizeBytes   int       `json:"size_bytes"`
	FileMode    int       `json:"file_mode"`
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
		zap.L().Error("failed to open metadata database connection")
		render.Status(r, http.StatusInternalServerError)
		return
	}

	// Get all the metadata
	query := `
		SELECT * FROM metadata
	`

	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query")
		render.Status(r, http.StatusBadRequest)
		return
	}

	metadata, err := RowsToMetadata(rows)
	if err != nil {
		zap.L().Error("failed to parse metadata")
		render.Status(r, http.StatusBadRequest)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, metadata)
}
