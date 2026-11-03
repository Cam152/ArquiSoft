// Package client es el único lugar de la CLI que habla con agr-gateway-or
// (conector c2). Cada método es una ruta de la API pública del gateway
// (README, sección 5.1).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIError es una respuesta de error del gateway o de un servicio detrás de él.
type APIError struct {
	Status int
	Detail string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d)", e.Detail, e.Status)
}

// ErrNoAdminKey: falta la llave para una ruta /admin.
var ErrNoAdminKey = errors.New("falta la llave de administración: configure AGORA_ADMIN_KEY o use --api-key")

// Client llama al gateway. Las rutas /admin/* llevan X-API-Key.
type Client struct {
	baseURL  string
	adminKey string
	http     *http.Client
}

func New(baseURL, adminKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		adminKey: adminKey,
		http:     &http.Client{Timeout: timeout},
	}
}

// ---------- Tipos de la API ----------

type Candidate struct {
	ID          int     `json:"id"`
	ElectionID  int     `json:"election_id"`
	Number      int     `json:"number"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type Election struct {
	ID             int         `json:"id"`
	Name           string      `json:"name"`
	Description    *string     `json:"description"`
	Status         string      `json:"status"`
	StartsAt       *time.Time  `json:"starts_at"`
	EndsAt         *time.Time  `json:"ends_at"`
	CreatedAt      time.Time   `json:"created_at"`
	IsOpen         bool        `json:"is_open"`
	CandidateCount int         `json:"candidate_count"`
	Candidates     []Candidate `json:"candidates"`
}

// ElectionInput es el cuerpo de crear y de editar. En la edición solo se envían
// los campos que no son nil.
type ElectionInput struct {
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
}

type CandidateInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type CandidateResult struct {
	CandidateID   int     `json:"candidate_id"`
	CandidateName string  `json:"candidate_name"`
	Votes         int     `json:"votes"`
	Percentage    float64 `json:"percentage"`
}

type Results struct {
	ElectionID   int               `json:"election_id"`
	ElectionName string            `json:"election_name"`
	TotalVotes   int               `json:"total_votes"`
	Results      []CandidateResult `json:"results"`
}

// AuditEvent es un evento de agr-audit-lo. Se guardan también los campos
// desconocidos para la salida --json.
type AuditEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	ElectionID int    `json:"election_id"`
	OccurredAt string `json:"occurred_at"`
	RecordedAt string `json:"recorded_at"`
}

// ---------- Elecciones ----------

func (c *Client) ListElections(ctx context.Context, status string) ([]Election, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	var out []Election
	return out, c.do(ctx, http.MethodGet, "/elections", q, nil, &out)
}

func (c *Client) GetElection(ctx context.Context, id int) (*Election, error) {
	var out Election
	return &out, c.do(ctx, http.MethodGet, "/elections/"+strconv.Itoa(id), nil, nil, &out)
}

func (c *Client) CreateElection(ctx context.Context, in ElectionInput) (*Election, error) {
	var out Election
	return &out, c.do(ctx, http.MethodPost, "/admin/elections", nil, in, &out)
}

func (c *Client) UpdateElection(ctx context.Context, id int, in ElectionInput) (*Election, error) {
	var out Election
	return &out, c.do(ctx, http.MethodPatch, "/admin/elections/"+strconv.Itoa(id), nil, in, &out)
}

func (c *Client) DeleteElection(ctx context.Context, id int) error {
	return c.do(ctx, http.MethodDelete, "/admin/elections/"+strconv.Itoa(id), nil, nil, nil)
}

// OpenElection pasa la elección de borrador a activa.
func (c *Client) OpenElection(ctx context.Context, id int) (*Election, error) {
	var out Election
	return &out, c.do(ctx, http.MethodPost, fmt.Sprintf("/admin/elections/%d/activate", id), nil, nil, &out)
}

// CloseElection pasa la elección de activa a cerrada.
func (c *Client) CloseElection(ctx context.Context, id int) (*Election, error) {
	var out Election
	return &out, c.do(ctx, http.MethodPost, fmt.Sprintf("/admin/elections/%d/close", id), nil, nil, &out)
}

// ---------- Candidatos ----------

func (c *Client) AddCandidate(ctx context.Context, electionID int, in CandidateInput) (*Candidate, error) {
	var out Candidate
	return &out, c.do(ctx, http.MethodPost, fmt.Sprintf("/admin/elections/%d/candidates", electionID), nil, in, &out)
}

func (c *Client) RemoveCandidate(ctx context.Context, electionID, candidateID int) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/admin/elections/%d/candidates/%d", electionID, candidateID), nil, nil, nil)
}

// ---------- Resultados y auditoría ----------

func (c *Client) Results(ctx context.Context, electionID int) (*Results, error) {
	var out Results
	return &out, c.do(ctx, http.MethodGet, "/results/"+strconv.Itoa(electionID), nil, nil, &out)
}

// Audit devuelve los eventos más recientes. electionID 0 significa todas.
func (c *Client) Audit(ctx context.Context, electionID, limit int) ([]AuditEvent, error) {
	q := url.Values{}
	if electionID > 0 {
		q.Set("election_id", strconv.Itoa(electionID))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/admin/audit", q, nil, &raw); err != nil {
		return nil, err
	}
	return decodeEvents(raw)
}

// decodeEvents acepta una lista, o un objeto con la lista en "events" o "items".
func decodeEvents(raw json.RawMessage) ([]AuditEvent, error) {
	var list []AuditEvent
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}
	var wrapped struct {
		Events []AuditEvent `json:"events"`
		Items  []AuditEvent `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("respuesta de auditoría inesperada: %w", err)
	}
	if wrapped.Events != nil {
		return wrapped.Events, nil
	}
	return wrapped.Items, nil
}

// ---------- Transporte ----------

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	admin := strings.HasPrefix(path, "/admin/")
	if admin && c.adminKey == "" {
		return ErrNoAdminKey
	}

	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "agora-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// La llave solo viaja en las rutas que la necesitan.
	if admin {
		req.Header.Set("X-API-Key", c.adminKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo conectar con el gateway en %s: %w", c.baseURL, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return &APIError{Status: res.StatusCode, Detail: detailOf(res.StatusCode, raw)}
	}
	if out == nil || res.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("respuesta inesperada del gateway: %w", err)
	}
	return nil
}

// detailOf lee {"detail": ...}. FastAPI devuelve en los 422 una lista de errores
// por campo; se resumen como "campo: mensaje".
func detailOf(status int, raw []byte) string {
	var body struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(raw, &body) == nil && len(body.Detail) > 0 {
		var text string
		if json.Unmarshal(body.Detail, &text) == nil && text != "" {
			return text
		}
		var fields []struct {
			Loc []any  `json:"loc"`
			Msg string `json:"msg"`
		}
		if json.Unmarshal(body.Detail, &fields) == nil && len(fields) > 0 {
			parts := make([]string, 0, len(fields))
			for _, f := range fields {
				if n := len(f.Loc); n > 0 {
					parts = append(parts, fmt.Sprintf("%v: %s", f.Loc[n-1], f.Msg))
				} else {
					parts = append(parts, f.Msg)
				}
			}
			return strings.Join(parts, "; ")
		}
	}
	return http.StatusText(status)
}
