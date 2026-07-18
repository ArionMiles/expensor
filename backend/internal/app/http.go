package app

import (
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/community"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/httpapi"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store/instrumented"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

type httpDependencies struct {
	config     config.App
	content    catalog.Content
	registry   *plugins.Registry
	llm        llmRuntime
	store      *instrumented.Store
	controller *daemon.Controller
	community  *community.Service
	logger     *slog.Logger
	logLevel   *slog.LevelVar
}

func newHTTPServer(deps httpDependencies) *httpapi.Server {
	handlers := httpapi.NewHandlers(httpapi.HandlersConfig{
		Registry: deps.registry, LLMRegistry: deps.llm.registry, LLMRouter: deps.llm.router,
		RuleDrafts: deps.llm.ruleDrafts, LLMScope: deps.llm.scope, Store: deps.store,
		Daemon: deps.controller, Community: deps.community, Version: config.Version,
		BaseURL: deps.config.BaseURL, FrontendURL: deps.config.FrontendURL, ThunderbirdDataDir: deps.config.Thunderbird.DataDir,
		ScanInterval: deps.config.ScanInterval, LookbackDays: deps.config.LookbackDays, BanksData: deps.content.BanksJSON,
		Logger: deps.logger.With("component", "api"), LogLevel: deps.logLevel,
	})
	return httpapi.NewServer(deps.config.Port, handlers, deps.config.StaticDir, deps.logger.With("component", "http"))
}
