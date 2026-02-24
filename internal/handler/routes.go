package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(h.recoverer)

	r.Get("/", h.Index)
	r.Get("/health", h.Health)
	r.Get("/doctor", h.Doctor)
	r.Handle("/assets/*", http.StripPrefix("/assets/", h.staticHandler()))

	// API utilitários
	r.Get("/api/scan-summary", h.ScanSummary)

	// SSH Accounts
	r.Get("/tools/ssh/accounts", h.ListAccounts)
	r.Post("/tools/ssh/accounts", h.CreateAccount)
	r.Get("/tools/ssh/accounts/new", h.NewAccountDrawer)
	r.Get("/tools/ssh/accounts/{id}/edit", h.EditAccountDrawer)
	r.Post("/tools/ssh/accounts/{id}", h.UpdateAccount)
	r.Delete("/tools/ssh/accounts/{id}", h.DeleteAccount)
	r.Post("/tools/ssh/accounts/{id}/apply-ssh", h.ApplySSHConfig)
	r.Post("/tools/ssh/accounts/{id}/test", h.TestConnection)
	r.Post("/tools/ssh/accounts/{id}/preview-apply", h.PreviewApplySSHConfig)

	// SSH Config Import
	r.Get("/tools/ssh/import", h.ImportSSHConfigDrawer)
	r.Post("/tools/ssh/import/validate", h.ValidateSSHConfigPath)
	r.Post("/tools/ssh/import", h.ImportSSHAccounts)

	// Key Manager
	r.Get("/tools/keys", h.ListKeys)
	r.Get("/tools/keys/new", h.NewKeyDrawer)
	r.Post("/tools/keys", h.CreateKey)
	r.Delete("/tools/keys/{id}", h.DeleteKey)
	r.Post("/tools/keys/{id}/regen-pub", h.RegenPublicKey)
	r.Get("/tools/keys/{id}/export", h.ExportKeyBase64)
	r.Get("/tools/keys/import", h.ImportKeysDrawer)
	r.Post("/tools/keys/import/validate", h.ValidateImportPath)
	r.Post("/tools/keys/import", h.ImportKeys)

	// Repositórios
	r.Get("/tools/repos", h.Repositories)
	r.Get("/tools/repos/clone/new", h.NewCloneDrawer)
	r.Post("/tools/repos/clone", h.StartCloneJob)
	r.Get("/tools/repos/jobs/{id}", h.CloneJobStatus)
	r.Delete("/tools/repos/{id}", h.DeleteRepository)
	r.Get("/tools/repos/{id}/status", h.RepoStatus)
	// Scan
	r.Get("/tools/repos/scan", h.ScanReposDrawer)
	r.Post("/tools/repos/scan/validate", h.ValidateScanPath)
	r.Post("/tools/repos/scan/import", h.ImportScannedRepos)
	// Pull
	r.Post("/tools/repos/{id}/pull", h.StartPullJob)
	r.Get("/tools/repos/pull-jobs/{jobId}", h.PullJobStatus)
	// Branch + config + terminal
	r.Post("/tools/repos/{id}/branch", h.NewBranchHandler)
	r.Get("/tools/repos/{id}/tab/{tab}", h.GetRepoTab)
	r.Post("/tools/repos/{id}/git-config", h.SetRepoGitConfigHandler)
	r.Post("/tools/repos/{id}/terminal", h.OpenRepoTerminal)

	// Git Identities
	r.Get("/tools/git", h.ListIdentities)
	r.Get("/tools/git/identities/new", h.NewIdentityDrawer)
	r.Post("/tools/git/identities", h.CreateIdentity)
	r.Get("/tools/git/identities/{id}/edit", h.EditIdentityDrawer)
	r.Post("/tools/git/identities/{id}", h.UpdateIdentity)
	r.Delete("/tools/git/identities/{id}", h.DeleteIdentity)
	r.Get("/tools/git/global-config", h.GlobalConfigDrawer)
	r.Post("/tools/git/global-config", h.SaveGlobalConfig)
	r.Get("/tools/git/includeif", h.IncludeIfDrawer)
	r.Post("/tools/git/includeif", h.AddIncludeIfRule)
	r.Delete("/tools/git/includeif", h.RemoveIncludeIfRule)
	r.Get("/tools/git/identities/{id}/signing", h.SigningSetupDrawer)
	r.Post("/tools/git/identities/{id}/signing", h.ApplySigning)

	// Servers
	r.Get("/tools/servers", h.ListServers)
	r.Get("/tools/servers/new", h.NewServerDrawer)
	r.Post("/tools/servers", h.CreateServer)
	r.Get("/tools/servers/{id}/edit", h.EditServerDrawer)
	r.Post("/tools/servers/{id}", h.UpdateServer)
	r.Delete("/tools/servers/{id}", h.DeleteServer)
	r.Post("/tools/servers/{id}/test", h.StartTestJob)
	r.Get("/tools/servers/test-jobs/{jobId}", h.TestJobStatus)
	r.Post("/tools/servers/{id}/connect", h.ConnectServer)
	r.Get("/tools/servers/{id}/send-file", h.SendFileDrawer)
	r.Post("/tools/servers/{id}/send-file", h.StartSendFileJob)
	r.Get("/tools/servers/send-jobs/{jobId}", h.SendFileJobStatus)

	// System
	r.Get("/tools/system", h.SystemDashboard)
	r.Get("/tools/system/widgets", h.SystemWidgets)
	r.Post("/tools/system/hostname", h.SetHostname)

	// Docker
	r.Get("/tools/docker", h.DockerDashboard)
	r.Get("/tools/docker/status", h.DockerStatusPartial)
	r.Post("/tools/docker/start", h.StartDockerHandler)
	r.Get("/tools/docker/containers", h.ContainerList)
	r.Post("/tools/docker/containers/{id}/{action}", h.ContainerAction)
	r.Get("/tools/docker/containers/{id}/logs", h.ContainerLogs)
	r.Get("/tools/docker/containers/{id}", h.ContainerDetail)
	r.Get("/tools/docker/images", h.ListImages)
	r.Post("/tools/docker/images/pull", h.StartPullImage)
	r.Get("/tools/docker/pull-jobs/{id}", h.PullImageJobStatus)
	r.Delete("/tools/docker/images/{id}", h.RemoveImage)
	r.Get("/tools/docker/templates/new", h.TemplateDrawer)
	r.Post("/tools/docker/templates", h.LaunchTemplate)

	return r
}
