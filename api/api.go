package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/amalgamated-tools/astral-store/config"
	"github.com/amalgamated-tools/astral-store/models"
	sentrynegroni "github.com/getsentry/sentry-go/negroni"
	"github.com/gorilla/sessions"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/julienschmidt/httprouter"
	"github.com/urfave/negroni"
)

type Api struct {
	users       *models.UserManager
	config      *config.Config
	server      *http.Server
	log         hclog.InterceptLogger
	cookieStore *sessions.CookieStore
}

// New returns an initialized web.
func New(config *config.Config, log hclog.InterceptLogger) (api *Api, err error) {
	log = log.ResetNamed("web").(hclog.InterceptLogger)
	db := models.NewSqliteDB("data.db")
	usermgr, _ := models.NewUserManager(db)
	api = &Api{
		users:       usermgr,
		log:         log,
		config:      config,
		cookieStore: sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET"))),
	}

	// setup http server
	api.server = &http.Server{
		Addr:           config.Server.Address,
		Handler:        api.setupHandler(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return api, nil
}

func (api *Api) setupHandler() *negroni.Negroni {
	n := negroni.Classic()

	recovery := negroni.NewRecovery()
	recovery.Formatter = &negroni.HTMLPanicFormatter{}
	n.Use(recovery)

	sentry := sentrynegroni.New(sentrynegroni.Options{
		Repanic:         true,
		WaitForDelivery: false,
	})
	n.Use(sentry)

	n.UseHandler(api.setupRoutes())

	return n
}

func (api *Api) setupRoutes() *httprouter.Router {
	router := httprouter.New()
	router.GET("/", api.Index)
	return router
}

func (api *Api) Start() error {
	api.log.Debug("Starting Web HTTP Server on", "address", api.server.Addr)

	// Start HTTP server
	if err := api.server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener.
		api.log.Error("Starting Web Server and received an error", "error", err)
		return err
	}
	api.log.Debug("Started Web HTTP Server", "address", api.server.Addr)
	return nil
}

func (api *Api) Shutdown() error {
	api.log.Debug("Gracefully shutting down the Web")

	var errs *multierror.Error

	if err := api.server.Shutdown(context.Background()); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}
