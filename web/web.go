package web

import (
	"context"
	"fmt"
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
	log = log.ResetNamed("api").(hclog.InterceptLogger)
	web = &Web{
		log:    log,
		config: config,
	}

	// setup http server
	web.server = &http.Server{
		Addr:           config.Server.Address,
		Handler:        web.setupRoutes(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return web, nil
}

func (web *Web) Index(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	session, _ := store.Get(req, "session-name")
	for k, v := range session.Values {
		fmt.Printf("k: %v\n", k)
		fmt.Printf("v: %v\n", v)
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
func (web *Web) setupRoutes() *negroni.Negroni {
	router := httprouter.New()
	router.GET("/", web.Index)

	n := negroni.Classic() // Includes some default middlewares
	recovery := negroni.NewRecovery()
	recovery.Formatter = &negroni.HTMLPanicFormatter{}
	n.Use(sentrynegroni.New(sentrynegroni.Options{
		Repanic:         true,
		WaitForDelivery: false,
	}))
	n.Use(recovery)
	n.UseHandler(router)

	return n
}

func (a *Web) Start() error {
	a.log.Debug("Starting Web HTTP Server", "address", a.server.Addr)

	// Start HTTP server
	if err := a.server.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener.
		a.log.Error("Starting Web Server and received an error", "error", err)
		return err
	}
	a.log.Debug("Started Web HTTP Server", "address", a.server.Addr)
	return nil
}

func (a *Web) Shutdown() error {
	a.log.Debug("Gracefully shutting down the Web")

	var errs *multierror.Error

	if err := a.server.Shutdown(context.Background()); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}
