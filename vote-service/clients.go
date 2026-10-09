package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Candidate y Election son la parte de la respuesta del Election Service
// que necesita este servicio (GET /elections/{id}).
type Candidate struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Election struct {
	ID         int         `json:"id"`
	IsOpen     bool        `json:"is_open"`
	Candidates []Candidate `json:"candidates"`
}

var errElectionNotFound = errors.New("la elección no existe")

// voterError es una respuesta del Voter Service que se devuelve tal cual al
// cliente (401 token inválido, 409 ya votó).
type voterError struct {
	Status int
	Detail string
}

func (e *voterError) Error() string { return e.Detail }

// Clients agrupa las llamadas REST a los otros dos servicios.
type Clients struct {
	http        *http.Client
	electionURL string
	voterURL    string
	serviceKey  string
}

func newClients(cfg Config) *Clients {
	return &Clients{
		http:        &http.Client{Timeout: 5 * time.Second},
		electionURL: cfg.ElectionServiceURL,
		voterURL:    cfg.VoterServiceURL,
		serviceKey:  cfg.ServiceAPIKey,
	}
}

// GetElection devuelve errElectionNotFound si no existe; cualquier otro error
// significa que el Election Service no respondió bien.
func (c *Clients) GetElection(ctx context.Context, id int) (*Election, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/elections/%d", c.electionURL, id), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, errElectionNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("election service respondió HTTP %d", res.StatusCode)
	}
	var election Election
	if err := json.NewDecoder(res.Body).Decode(&election); err != nil {
		return nil, err
	}
	return &election, nil
}

// MarkVoted marca al votante dueño del token. Devuelve *voterError con 401 o 409
// cuando el Voter Service rechaza la petición.
func (c *Clients) MarkVoted(ctx context.Context, authorization string) error {
	return c.voterCall(ctx, "/voters/mark-voted", authorization)
}

// UnmarkVoted es la compensación cuando no se pudo guardar el voto.
func (c *Clients) UnmarkVoted(ctx context.Context, authorization string) error {
	return c.voterCall(ctx, "/voters/unmark-voted", authorization)
}

func (c *Clients) voterCall(ctx context.Context, path, authorization string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.voterURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("X-Service-Key", c.serviceKey)

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusConflict:
		var body struct {
			Detail string `json:"detail"`
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		if json.Unmarshal(raw, &body) != nil || body.Detail == "" {
			body.Detail = http.StatusText(res.StatusCode)
		}
		return &voterError{Status: res.StatusCode, Detail: body.Detail}
	default:
		return fmt.Errorf("voter service respondió HTTP %d", res.StatusCode)
	}
}
