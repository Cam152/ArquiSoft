// Package client tiene los conectores REST hacia el Election Service y el Voter Service.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"vote-service/internal/model"
)

const timeout = 5 * time.Second

// ElectionClient llama al Election Service.
type ElectionClient struct {
	http    *http.Client
	baseURL string
}

func NewElectionClient(baseURL string) *ElectionClient {
	return &ElectionClient{http: &http.Client{Timeout: timeout}, baseURL: baseURL}
}

// GetElection devuelve model.ErrElectionNotFound si no existe; cualquier otro
// error significa que el Election Service no respondió bien.
func (c *ElectionClient) GetElection(ctx context.Context, id int) (*model.Election, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/elections/%d", c.baseURL, id), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, model.ErrElectionNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("election service respondió HTTP %d", res.StatusCode)
	}
	var election model.Election
	if err := json.NewDecoder(res.Body).Decode(&election); err != nil {
		return nil, err
	}
	return &election, nil
}
