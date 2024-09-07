package routes

import (
	"database/sql"
	"errors"
	"fsd/internal/config"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

type ProcResult struct {
	ID        int       `json:"id"`
	Stdout    string    `json:"stdout"`
	Stderr    string    `json:"stderr"`
	CreatedAt time.Time `json:"created_at"`
}

// Bind implements render.Binder.
func (p *ProcSubmitRequest) Bind(r *http.Request) error {
	if p.Command == "" {
		return errors.New("command is required")
	}

	return nil
}

func (p *ProcController) GetProc(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
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
	db, err := sql.Open("sqlite3", config.GetDBPath())
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
	result, err := db.Exec(query, req.Command, strings.Join(req.Args, " "), time.Now())
	if err != nil {
		zap.L().Error("failed to insert proc", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to insert proc"))
		return
	}

	// Get the ID of the inserted proc
	id, err := result.LastInsertId()
	if err != nil {
		zap.L().Error("failed to get last insert id", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to get last insert id"))
		return
	}

	proc := Proc{
		ID:         int(id),
		Command:    req.Command,
		Args:       strings.Join(req.Args, " "),
		IsExecuted: 0,
		CreatedAt:  time.Now(),
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, proc)
}

func (p *ProcController) GetProcResults(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Error("failed to open proc database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to open proc database connection"))
		return
	}
	defer db.Close()

	query := `SELECT * FROM proc_results`
	rows, err := db.Query(query)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to send database query"))
		return
	}
	defer rows.Close()

	var results []ProcResult
	for rows.Next() {
		var result ProcResult
		err = rows.Scan(&result.ID, &result.Stdout, &result.Stderr, &result.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc result", zap.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to scan proc result"))
			return
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc result rows", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "error iterating over proc result rows"))
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, results)
}

func (p *ProcController) GetProcResult(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Error("failed to open proc database connection", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to open proc database connection"))
		return
	}
	defer db.Close()

	id := chi.URLParam(r, "id")
	if id == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, NewErrorResponse(http.StatusBadRequest, "id is required"))
		return
	}

	query := `SELECT * FROM proc_results WHERE id = ?`
	rows, err := db.Query(query, id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to send database query"))
		return
	}
	defer rows.Close()

	var results []ProcResult
	for rows.Next() {
		var result ProcResult
		err = rows.Scan(&result.ID, &result.Stdout, &result.Stderr, &result.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc result", zap.Error(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "failed to scan proc result"))
			return
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc result rows", zap.Error(err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, NewErrorResponse(http.StatusInternalServerError, "error iterating over proc result rows"))
		return
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, results)
}
