package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, h *Handler) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pong"))
		})

		r.Get("/auth/github/login", h.githubLogin)
		r.Get("/auth/github/callback", h.githubCallback)
		r.Get("/auth/github/app/install", h.githubAppInstall)
		r.Get("/auth/github/app/setup", h.githubAppSetup)

		r.Group(func(r chi.Router) {
			r.Use(h.authMiddleware)
			r.Get("/me", h.me)
			r.Get("/github/repositories", h.githubRepositories)
			r.Get("/github/repositories/{repoID}", h.githubRepositoryByID)
			r.Get("/github/repositories/{repoID}/dependency-files", h.githubRepositoryDependencyFiles)
			r.Get("/github/repositories/{repoID}/dependencies", h.githubRepositoryDependencies)
			r.Post("/github/repositories/{repoID}/connect", h.connectGitHubRepository)
		})
	})
}
