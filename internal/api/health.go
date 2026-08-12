package api

import (
	"net/http"
)

type Pinger interface {
}

func (a *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (a *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// TODO
}
