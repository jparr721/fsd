package routes

import "github.com/go-chi/chi/v5"

func MakeRouter(r chi.Router) {
	r.Route("/healthz", func(r chi.Router) {
		r.Get("/", Healthz)
	})

	r.Route("/metadata", func(r chi.Router) {
		ctrl := MetadataController{}
		r.Get("/", ctrl.GetMetadata)
	})
}
