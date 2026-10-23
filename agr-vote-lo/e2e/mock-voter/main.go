// mock-voter simula el Voter Service en las pruebas de punta a punta, mientras
// el servicio real (Java) no exista. Cumple el contrato de la sección 6.1 del
// README: cada token distinto es un votante, y el token "invalid" se rechaza.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type voters struct {
	mu         sync.Mutex
	voted      map[string]bool
	serviceKey string
}

// authenticate devuelve el token del votante, o "" si la petición no es válida.
func (v *voters) authenticate(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" || token == "invalid" || r.Header.Get("X-Service-Key") != v.serviceKey {
		return ""
	}
	return token
}

func (v *voters) markVoted(w http.ResponseWriter, r *http.Request) {
	token := v.authenticate(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Token inválido o expirado"})
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.voted[token] {
		writeJSON(w, http.StatusConflict, map[string]string{"detail": "El votante ya votó"})
		return
	}
	v.voted[token] = true
	writeJSON(w, http.StatusOK, map[string]string{"status": "marked"})
}

func (v *voters) unmarkVoted(w http.ResponseWriter, r *http.Request) {
	token := v.authenticate(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "Token inválido o expirado"})
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.voted, token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "unmarked"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func main() {
	serviceKey := os.Getenv("SERVICE_API_KEY")
	if serviceKey == "" {
		serviceKey = "dev-service-key"
	}
	v := &voters{voted: map[string]bool{}, serviceKey: serviceKey}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /voters/mark-voted", v.markVoted)
	mux.HandleFunc("POST /voters/unmark-voted", v.unmarkVoted)

	log.Println("mock-voter escuchando en :8001")
	log.Fatal(http.ListenAndServe(":8001", mux))
}
