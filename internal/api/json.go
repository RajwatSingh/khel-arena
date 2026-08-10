package api

import (
	"encoding/json"
	"net/http"

	"github.com/RajwatSingh/khel-arena/internal/domain"
)

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T 

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if 	err := decoder.Decode(&v); err != nil {
		return v, domain.Invalid("", "Malformed request body.")
	}

	return v, nil
}

func encode(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
