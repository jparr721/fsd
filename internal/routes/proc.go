package routes

import (
	"database/sql"
	"errors"
	"fsd/pkg/tasks"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"go.uber.org/zap"
)

type ProcController struct{}
type Proc struct {
	ID         int       `json:"id"`
	Command    string    `json:"command"`
	Args       string    `json:"args"`
	IsExecuted int       `json:"is_executed"`
	CreatedAt  time.Time `json:"created_at"`
}

type ProcSubmitRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Bind implements render.Binder.
func (p *ProcSubmitRequest) Bind(r *http.Request) error {
	if p.Command == "" {
		return errors.New("command is required")
	}

	return nil
}

func (p *ProcController) GetProc(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", tasks.FSD_DB_FILENAME)
	if err != nil {
		zap.L().Error("failed to open proc database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := `SELECT * FROM proc ORDER BY created_at DESC`
	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to send database query"))
		return
	}
	defer rows.Close()

	var procs []Proc
	for rows.Next() {
		var proc Proc
		err = rows.Scan(&proc.ID, &proc.Command, &proc.Args, &proc.IsExecuted, &proc.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc", zap.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to scan proc"))
			return
		}
		procs = append(procs, proc)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc rows", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "error iterating over proc rows"))
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, procs)
}

func (p *ProcController) SubmitProc(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", tasks.FSD_DB_FILENAME)
	if err != nil {
		zap.L().Error("failed to open proc database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to open proc database connection"))
		return
	}
	defer db.Close()

	var req ProcSubmitRequest
	if err := render.Bind(r, &req); err != nil {
		zap.L().Error("failed to bind request", zap.Error(err))
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, NewErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	query := `INSERT INTO proc (command, args, created_at) VALUES (?, ?, ?)	`
	_, err = db.Exec(query, req.Command, strings.Join(req.Args, " "), time.Now())
	if err != nil {
		zap.L().Error("failed to insert proc", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to insert proc"))
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, nil)
}
