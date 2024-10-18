package routes

import (
	"errors"
	"fsd/pkg/procs"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
)

type RefreshController struct{}

type Refresh struct {
	ID         int       `json:"id"`
	Command    string    `json:"command"`
	Args       string    `json:"args"`
	IsExecuted int       `json:"is_executed"`
	CreatedAt  time.Time `json:"created_at"`
}

type RefreshRequest struct {
	URL         string `json:"url"`
	PlaylistEnd int    `json:"playlist_end"`
}

func (c *RefreshRequest) bindHelper() error {
	if c.URL == "" {
		return errors.New("url is required")
	}

	if c.PlaylistEnd == 0 {
		return errors.New("playlist_end is required and must be a positive integer")
	}

	return nil
}

func (c *RefreshRequest) Bind(r *http.Request) error {
	return c.bindHelper()
}

func (c *RefreshController) Refresh(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	playlistEnd, err := strconv.Atoi(r.URL.Query().Get("playlist_end"))
	if err != nil {
		http.Error(w, "Invalid playlist end", http.StatusBadRequest)
		return
	}

	proc, err := procs.NewRefreshProc(r.Context(), url, playlistEnd)
	if err != nil {
		http.Error(w, "Failed to create refresh proc", http.StatusInternalServerError)
		return
	}

	render.JSON(w, r, proc)
}
