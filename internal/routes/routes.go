package routes

import "github.com/go-chi/chi/v5"

func MakeRouter(r chi.Router) {
	r.Route("/healthz", func(r chi.Router) {
		r.Get("/", Healthz)
	})

	r.Route("/metadata", func(r chi.Router) {
		ctrl := MetadataController{}
		r.Get("/", ctrl.GetMetadata)
		r.Get("/latest", ctrl.GetLatestMetadata)
	})

	r.Route("/disk", func(r chi.Router) {
		ctrl := DiskController{}
		r.Get("/", ctrl.GetDiskStats)
		r.Get("/latest", ctrl.GetLatestDiskStats)
	})

	r.Route("/proc", func(r chi.Router) {
		ctrl := ProcController{}
		r.Get("/", ctrl.GetProc)
		r.Post("/", ctrl.SubmitProc)
		r.Get("/results", ctrl.GetProcResults)
		r.Get("/results/{id}", ctrl.GetProcResult)
	})
}
