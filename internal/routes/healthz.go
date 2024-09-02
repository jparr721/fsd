package routes

import (
	"net/http"

	"github.com/go-chi/render"
)

type HealthzResponse struct {
	Status string `json:"status"`
}

func (*HealthzResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func Healthz(w http.ResponseWriter, r *http.Request) {
	resp := &HealthzResponse{
		Status: "ok",
	}

	render.Status(r, http.StatusOK)
	render.JSON(w, r, resp)
}
