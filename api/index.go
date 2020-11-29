package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (api *Api) Index(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	session, _ := api.cookieStore.Get(req, "session-name")
	for k, v := range session.Values {
		api.log.Debug("Session details", "key", k, "value", v)
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
