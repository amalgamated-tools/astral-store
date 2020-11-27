package web

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/amalgamated-tools/astral-store/config"
	sentrynegroni "github.com/getsentry/sentry-go/negroni"
	"github.com/gorilla/sessions"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/julienschmidt/httprouter"
	"github.com/urfave/negroni"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))

type Web struct {
	config *config.Config
	server *http.Server
	log    hclog.InterceptLogger
}

// New returns an initialized web.
func New(config *config.Config, log hclog.InterceptLogger) (web *Web, err error) {
	log = log.ResetNamed("web").(hclog.InterceptLogger)
	web = &Web{
		log:    log,
		config: config,
	}

	// setup http server
	web.server = &http.Server{
		Addr:           config.Server.Address,
		Handler:        web.setupHandler(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return web, nil
}

func (web *Web) Index(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	session, _ := store.Get(req, "session-name")
	for k, v := range session.Values {
		web.log.Debug("Session details", "key", k, "value", v)
	}
	// Set some session values.
	session.Values["foo"] = "bar"
	session.Values[42] = 43
	// Save it before we write to the response/return from the handler.
	err := session.Save(req, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (web *Web) setupHandler() *negroni.Negroni {
	n := negroni.Classic()

	recovery := negroni.NewRecovery()
	recovery.Formatter = &negroni.HTMLPanicFormatter{}
	n.Use(recovery)

	sentry := sentrynegroni.New(sentrynegroni.Options{
		Repanic:         true,
		WaitForDelivery: false,
	})
	n.Use(sentry)

	n.UseHandler(web.setupRoutes())

	return n
}

func (web *Web) setupRoutes() *httprouter.Router {
	router := httprouter.New()
	router.GET("/", web.Index)
	return router
}

func (web *Web) Start() error {
	web.log.Debug("Starting Web HTTP Server on", "address", web.server.Addr)

	// Start HTTP server
	if err := web.server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener.
		web.log.Error("Starting Web Server and received an error", "error", err)
		return err
	}
	web.log.Debug("Started Web HTTP Server", "address", web.server.Addr)
	return nil
}

func (web *Web) Shutdown() error {
	web.log.Debug("Gracefully shutting down the Web")

	var errs *multierror.Error

	if err := web.server.Shutdown(context.Background()); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}
