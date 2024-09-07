package routes

import (
	"database/sql"
	"fsd/internal/config"
	"net/http"
	"time"

	"github.com/go-chi/render"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type DiskController struct{}

type Disk struct {
	ID        int64     `json:"id"`
	Free      int64     `json:"free"`
	Available int64     `json:"available"`
	Size      int64     `json:"size"`
	Used      int64     `json:"used"`
	UsedPct   float64   `json:"used_pct"`
	CreatedAt time.Time `json:"created_at"`
}

func RowsToDiskStats(rows *sql.Rows) ([]Disk, error) {
	var disks []Disk

	for rows.Next() {
		var disk Disk
		if err := rows.Scan(
			&disk.ID,
			&disk.Free,
			&disk.Available,
			&disk.Size,
			&disk.Used,
			&disk.UsedPct,
			&disk.CreatedAt,
		); err != nil {
			return nil, err
		}

		disks = append(disks, disk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return disks, nil
}

func (d *DiskController) GetDiskStats(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Error("failed to open disk stats database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := `SELECT * FROM disk_stats ORDER BY created_at DESC`
	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	diskStats, err := RowsToDiskStats(rows)
	if err != nil {
		zap.L().Error("failed to parse disk stats", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, diskStats)
}

func (d *DiskController) GetLatestDiskStats(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Error("failed to open disk stats database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := `SELECT * FROM disk_stats ORDER BY created_at DESC LIMIT 1`
	row := db.QueryRow(query)

	var disk Disk
	err = row.Scan(
		&disk.ID,
		&disk.Free,
		&disk.Available,
		&disk.Size,
		&disk.Used,
		&disk.UsedPct,
		&disk.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			zap.L().Warn("no disk stats found")
			render.Status(r, http.StatusNotFound)
			return
		}
		zap.L().Error("failed to scan disk stats", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, disk)
}
