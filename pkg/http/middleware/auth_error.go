package middleware

import (
	"encoding/json"
	"net/http"
)

type authErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeAuthError(w http.ResponseWriter, status int, code string, message string, wwwAuthenticate string) {
	if wwwAuthenticate != "" {
		w.Header().Set("WWW-Authenticate", wwwAuthenticate)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(authErrorBody{
		Error: message,
		Code:  code,
	})
}
