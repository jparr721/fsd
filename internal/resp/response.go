package resp

import (
	"net/http"

	"github.com/go-chi/render"
)

type HttpResponse struct {
	Data    interface{} `json:"data"`
	Code    int         `json:"code"`
	Message string      `json:"message"`
}

func NewHttpResponse(code int, message string, data interface{}) HttpResponse {
	return HttpResponse{
		Data:    data,
		Code:    code,
		Message: message,
	}
}

func NewSuccessResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.Status(r, http.StatusOK)
	render.JSON(w, r, NewHttpResponse(http.StatusOK, "success", data))
}

func NewCreatedResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, NewHttpResponse(http.StatusCreated, "created", data))
}

func NewErrorResponse(w http.ResponseWriter, r *http.Request, code int, message string) {
	render.Status(r, code)
	render.JSON(w, r, NewHttpResponse(code, message, nil))
}

func NewBadRequestResponse(w http.ResponseWriter, r *http.Request, message string) {
	NewErrorResponse(w, r, http.StatusBadRequest, message)
}

func NewInternalServerErrorResponse(w http.ResponseWriter, r *http.Request, message string) {
	NewErrorResponse(w, r, http.StatusInternalServerError, message)
}
