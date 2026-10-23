package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"vote-service/internal/model"
)

// VoterClient llama al Voter Service.
type VoterClient struct {
	http       *http.Client
	baseURL    string
	serviceKey string
}

func NewVoterClient(baseURL, serviceKey string) *VoterClient {
	return &VoterClient{http: &http.Client{Timeout: timeout}, baseURL: baseURL, serviceKey: serviceKey}
}

// MarkVoted marca al votante dueño del token. Devuelve *model.VoterRejection
// cuando el Voter Service rechaza la petición (401 token inválido, 409 ya votó).
func (c *VoterClient) MarkVoted(ctx context.Context, authorization string) error {
	return c.call(ctx, "/voters/mark-voted", authorization)
}

// UnmarkVoted es la compensación cuando no se pudo guardar el voto.
func (c *VoterClient) UnmarkVoted(ctx context.Context, authorization string) error {
	return c.call(ctx, "/voters/unmark-voted", authorization)
}

func (c *VoterClient) call(ctx context.Context, path, authorization string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
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
		reason := model.InvalidToken
		if res.StatusCode == http.StatusConflict {
			reason = model.AlreadyVoted
		}
		return &model.VoterRejection{Reason: reason, Detail: body.Detail}
	default:
		return fmt.Errorf("voter service respondió HTTP %d", res.StatusCode)
	}
}
