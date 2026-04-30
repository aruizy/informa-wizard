package monday

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const mondayAPIURL = "https://api.monday.com/v2"
const validateTimeout = 5 * time.Second

// ValidateToken verifies that the given Monday.com API token is valid by
// calling the Monday GraphQL API with a lightweight query.
// Returns nil on success, or a descriptive error on failure.
// If token is empty, ValidateToken returns nil (no-op).
func ValidateToken(token string) error {
	if token == "" {
		return nil
	}

	body := []byte(`{"query":"{ me { id name } }"}`)
	req, err := http.NewRequest(http.MethodPost, mondayAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: validateTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d — check your token", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Data   *struct{ Me *struct{ ID any `json:"id"` } } `json:"data"`
		Errors []any                                        `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("token rejected by Monday API")
	}

	if result.Data == nil || result.Data.Me == nil {
		return fmt.Errorf("unexpected API response — token may be invalid")
	}

	return nil
}
