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
			r.Get("/integrations", h.listIntegrations)
			r.Get("/integrations/activities", h.listIntegrationActivities)
			r.Get("/integrations/{provider}", h.getIntegration)
			r.Put("/integrations/{provider}/connect", h.connectIntegration)
			r.Post("/integrations/linear/teams", h.listLinearTeams)
			r.Post("/integrations/{provider}/messages", h.sendIntegrationMessage)
			r.Post("/integrations/{provider}/issues", h.createIntegrationIssue)
			r.Get("/policies", h.listPolicies)
			r.Post("/policies", h.createPolicy)
			r.Get("/policies/{policyID}", h.getPolicy)
			r.Put("/policies/{policyID}", h.updatePolicy)
			r.Post("/policies/{policyID}/scans/run", h.runPolicyScan)
			r.Patch("/policies/{policyID}/enabled", h.setPolicyEnabled)
			r.Delete("/policies/{policyID}", h.deletePolicy)
			r.Get("/scans", h.listScans)
			r.Get("/scans/{scanID}", h.getScan)
			r.Get("/findings", h.listFindings)
			r.Get("/findings/{findingID}", h.getFinding)
			r.Get("/system-health/summary", h.systemHealthSummary)
			r.Get("/system-health/services", h.systemHealthServices)
			r.Get("/system-health/queues", h.systemHealthQueues)
			r.Get("/system-health/logs", h.systemHealthLogs)
			r.Get("/system-health/logs/stream", h.systemHealthLogsStream)
			r.Get("/system-health/logs/services", h.systemHealthLogServices)
			r.Get("/domains", h.listCustomDomains)
			r.Post("/domains", h.createCustomDomain)
			r.Post("/domains/{domainID}/verify", h.verifyCustomDomain)
			r.Delete("/domains/{domainID}", h.deleteCustomDomain)
			r.Get("/github/repositories", h.githubRepositories)
			r.Get("/github/repositories/{repoID}", h.githubRepositoryByID)
			r.Get("/github/repositories/{repoID}/dependency-files", h.githubRepositoryDependencyFiles)
			r.Get("/github/repositories/{repoID}/dependencies", h.githubRepositoryDependencies)
			r.Post("/github/repositories/{repoID}/dependencies/fetch", h.fetchRepositoryDependencies)
			r.Post("/github/repositories/{repoID}/scans/run", h.runRepositoryScan)
			r.Get("/github/repositories/{repoID}/scans", h.repositoryScans)
			r.Get("/github/repositories/{repoID}/findings", h.repositoryFindings)
			r.Get("/github/repositories/{repoID}/policy", h.getRepositoryPolicy)
			r.Put("/github/repositories/{repoID}/policy", h.assignRepositoryPolicy)
			r.Delete("/github/repositories/{repoID}/policy", h.unassignRepositoryPolicy)
			r.Post("/github/repositories/{repoID}/connect", h.connectGitHubRepository)
		})
	})
}
