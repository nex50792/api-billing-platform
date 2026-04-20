package handler

import (
	"net/http"
)

type Pinger interface {
	Ping() error
}

func HealthHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("db NG"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
