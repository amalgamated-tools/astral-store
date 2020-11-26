package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/julienschmidt/httprouter"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"github.com/urfave/negroni"
)

var log hclog.Logger

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	m := make(map[string]string)
	m["google"] = "Google"
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	providerIndex := &ProviderIndex{Providers: keys, ProvidersMap: m}
	t, _ := template.New("foo").Parse(indexTemplate)
	t.Execute(w, providerIndex)
}

func Auth(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return p.ByName("provider"), nil
	}
	if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
		t, _ := template.New("foo").Parse(userTemplate)
		t.Execute(res, gothUser)
	} else {
		gothic.BeginAuthHandler(res, req)
	}
}

func Callback(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	user, err := gothic.CompleteUserAuth(res, req)
	if err != nil {
		fmt.Fprintln(res, err)
		return
	}
	t, _ := template.New("foo").Parse(userTemplate)
	t.Execute(res, user)
}

var indexTemplate = `{{range $key,$value:=.Providers}}
    <p><a href="/auth/{{$value}}">Log in with {{index $.ProvidersMap $value}}</a></p>
{{end}}`

func main() {

	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_KEY"), os.Getenv("GOOGLE_SECRET"), "http://localhost:8080/auth/google/callback"),
	)

	println("Starting")
	router := httprouter.New()
	router.GET("/", Index)
	router.GET("/auth/:provider", Auth)
	router.GET("/auth/:provider/callback", Callback)

	n := negroni.Classic() // Includes some default middlewares

	recovery := negroni.NewRecovery()
	recovery.Formatter = &negroni.HTMLPanicFormatter{}
	n.Use(recovery)
	n.UseHandler(router)

	s := &http.Server{
		Addr:           ":8080",
		Handler:        n,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	s.ListenAndServe()
}

type ProviderIndex struct {
	Providers    []string
	ProvidersMap map[string]string
}

var userTemplate = `
<p><a href="/logout/{{.Provider}}">logout</a></p>
<p>Name: {{.Name}} [{{.LastName}}, {{.FirstName}}]</p>
<p>Email: {{.Email}}</p>
<p>NickName: {{.NickName}}</p>
<p>Location: {{.Location}}</p>
<p>AvatarURL: {{.AvatarURL}} <img src="{{.AvatarURL}}"></p>
<p>Description: {{.Description}}</p>
<p>UserID: {{.UserID}}</p>
<p>AccessToken: {{.AccessToken}}</p>
<p>ExpiresAt: {{.ExpiresAt}}</p>
<p>RefreshToken: {{.RefreshToken}}</p>
`
