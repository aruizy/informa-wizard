package monday

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const mondayAPIURL = "https://api.monday.com/v2"
const validateTimeout = 10 * time.Second

// ErrNetwork wraps any network-related failure during token validation.
// Callers can choose to treat this as non-fatal (e.g. save anyway, warn user).
var ErrNetwork = errors.New("network unreachable")

// IsNetworkError reports whether err is a network failure during validation.
func IsNetworkError(err error) bool {
	return errors.Is(err, ErrNetwork)
}

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
		return fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Try to extract Monday's error message even on non-200 responses.
	var result struct {
		Data *struct {
			Me *struct {
				ID any `json:"id"`
			}
		} `json:"data"`
		Errors []struct {
			Message    string `json:"message"`
			Extensions struct {
				Code string `json:"code"`
			} `json:"extensions"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(respBody, &result)

	if len(result.Errors) > 0 {
		msg := result.Errors[0].Message
		code := result.Errors[0].Extensions.Code
		if code == "NOT_AUTHENTICATED" || resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("invalid token — Monday rejected it as not authenticated")
		}
		if msg != "" {
			return fmt.Errorf("Monday API: %s", msg)
		}
		return fmt.Errorf("Monday API rejected the request")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Monday API returned status %d — check your token", resp.StatusCode)
	}

	if result.Data == nil || result.Data.Me == nil {
		return fmt.Errorf("unexpected API response — token may be invalid")
	}

	return nil
}
