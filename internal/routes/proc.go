package routes

import (
	"database/sql"
	"errors"
	"fmt"
	"fsd/internal/resp"
	"fsd/pkg/procs"
	"net/http"
	"slices"
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
	Command string              `json:"command"`
	Args    map[string][]string `json:"args"`
}

type ProcResult struct {
	ID        int       `json:"id"`
	Stdout    string    `json:"stdout"`
	Stderr    string    `json:"stderr"`
	CreatedAt time.Time `json:"created_at"`
}

var PROCS = []string{
	procs.YtProcName(),
	procs.DirProcName(),
}

func (p *ProcSubmitRequest) bindHelper() error {
	switch p.Command {
	case procs.YtProcName():
		val, ok := p.Args["url"]
		if !ok {
			return errors.New("url is required")
		}

		if len(val) == 0 || val[0] == "" {
			return errors.New("non-empty url is required")
		}

		channelName, ok := p.Args["channel-name"]
		if !ok {
			return errors.New("channel-name is required")
		}

		if len(channelName) == 0 || channelName[0] == "" {
			return errors.New("non-empty channel-name is required")
		}
	case procs.DirProcName():
		val, ok := p.Args["dirname"]
		if !ok {
			return errors.New("dirname is required")
		}

		if len(val) == 0 {
			return errors.New("non-empty dirname required")
		}
	default:
		return errors.New("invalid proc choice")
	}

	return nil
}

// Bind implements render.Binder.
func (p *ProcSubmitRequest) Bind(r *http.Request) error {
	if p.Command == "" {
		return errors.New("command is required")
	}

	return p.bindHelper()
}

func (p *ProcController) GetAvailableProcs(w http.ResponseWriter, r *http.Request) {
	resp.NewSuccessResponse(w, r, PROCS)
}

func (p *ProcController) GetProcs(w http.ResponseWriter, r *http.Request) {
	db := r.Context().Value("db").(*sql.DB)

	query := `SELECT * FROM proc ORDER BY created_at DESC`
	rows, err := db.Query(query)
	if err != nil {
		zap.L().Error("failed to send database query", zap.Error(err))
		resp.NewInternalServerErrorResponse(w, r, "failed to send database query")
		return
	}
	defer rows.Close()

	procs := []Proc{}
	for rows.Next() {
		var proc Proc
		err = rows.Scan(&proc.ID, &proc.Command, &proc.Args, &proc.IsExecuted, &proc.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc", zap.Error(err))
			resp.NewInternalServerErrorResponse(w, r, "failed to scan proc")
			return
		}
		procs = append(procs, proc)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc rows", zap.Error(err))
		resp.NewInternalServerErrorResponse(w, r, "error iterating over proc rows")
		return
	}

	resp.NewSuccessResponse(w, r, procs)
}

func (p *ProcController) SubmitProc(w http.ResponseWriter, r *http.Request) {
	var req ProcSubmitRequest
	if err := render.Bind(r, &req); err != nil {
		zap.L().Error("failed to bind request", zap.Error(err))
		resp.NewBadRequestResponse(w, r, err.Error())
		return
	}

	if !slices.Contains(PROCS, req.Command) {
		resp.NewErrorResponse(w, r, http.StatusBadRequest, fmt.Sprintf("invalid proc: %s, wanted one of %s", req.Command, strings.Join(PROCS, ", ")))
		return
	}

	switch req.Command {
	case procs.YtProcName():
		ytProc, err := procs.NewYtProc(r.Context(), req.Args)
		if err != nil {
			zap.L().Error("failed to create yt proc", zap.String("proc", procs.YtProcName()), zap.Error(err))
			resp.NewInternalServerErrorResponse(w, r, "failed to create yt proc")
			return
		}

		proc := Proc{
			ID:         ytProc.GetID(),
			Command:    ytProc.GetCmd(),
			Args:       strings.Join(ytProc.GetArgs(), " "),
			IsExecuted: 0,
			CreatedAt:  time.Now(),
		}

		resp.NewCreatedResponse(w, r, proc)
	case procs.DirProcName():
		dirProc, err := procs.NewDirProc(r.Context(), req.Args["dirname"][0])
		if err != nil {
			zap.L().Error("failed to create proc", zap.String("proc", procs.DirProcName()), zap.Error(err))
			resp.NewInternalServerErrorResponse(w, r, "failed to create proc")
			return
		}

		proc := Proc{
			ID:         dirProc.GetID(),
			Command:    dirProc.GetCmd(),
			Args:       dirProc.GetArgs()[0],
			IsExecuted: 0,
			CreatedAt:  time.Now(),
		}
		resp.NewCreatedResponse(w, r, proc)
	}
}

func (p *ProcController) GetProcResults(w http.ResponseWriter, r *http.Request) {
	db := r.Context().Value("db").(*sql.DB)

	query := `SELECT * FROM proc_results`
	rows, err := db.Query(query)
	if err != nil {
		resp.NewInternalServerErrorResponse(w, r, "failed to send database query")
		return
	}
	defer rows.Close()

	var results []ProcResult
	for rows.Next() {
		var result ProcResult
		err = rows.Scan(&result.ID, &result.Stdout, &result.Stderr, &result.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc result", zap.Error(err))
			resp.NewInternalServerErrorResponse(w, r, "failed to scan proc result")
			return
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc result rows", zap.Error(err))
		resp.NewInternalServerErrorResponse(w, r, "error iterating over proc result rows")
		return
	}

	resp.NewSuccessResponse(w, r, results)
}

func (p *ProcController) GetProcResult(w http.ResponseWriter, r *http.Request) {
	db := r.Context().Value("db").(*sql.DB)

	id := chi.URLParam(r, "id")
	if id == "" {
		resp.NewBadRequestResponse(w, r, "id is required")
		return
	}

	query := `SELECT * FROM proc_results WHERE id = ?`
	rows, err := db.Query(query, id)
	if err != nil {
		resp.NewInternalServerErrorResponse(w, r, "failed to send database query")
		return
	}
	defer rows.Close()

	var results []ProcResult
	for rows.Next() {
		var result ProcResult
		err = rows.Scan(&result.ID, &result.Stdout, &result.Stderr, &result.CreatedAt)
		if err != nil {
			zap.L().Error("failed to scan proc result", zap.Error(err))
			resp.NewInternalServerErrorResponse(w, r, "failed to scan proc result")
			return
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		zap.L().Error("error iterating over proc result rows", zap.Error(err))
		resp.NewInternalServerErrorResponse(w, r, "error iterating over proc result rows")
		return
	}

	resp.NewSuccessResponse(w, r, results)
}
