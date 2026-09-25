package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var errUnauthorized = errors.New("invalid session")
var errAuthUnavailable = errors.New("authentication unavailable")

type Auth struct {
	URL    string
	Key    string
	Client *http.Client
}

// Supabase validates signature, expiry and account status. JWT claims received
// from a client are never trusted without this server-to-server verification.
func (a *Auth) user(ctx context.Context, authorization string) (string, error) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errUnauthorized
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(a.URL, "/")+"/auth/v1/user", nil)
	if err != nil {
		return "", errAuthUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+parts[1])
	req.Header.Set("apikey", a.Key)
	res, err := a.Client.Do(req)
	if err != nil {
		return "", errAuthUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return "", errUnauthorized
	}
	if res.StatusCode != http.StatusOK {
		return "", errAuthUnavailable
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&user); err != nil || !uuidPattern.MatchString(user.ID) {
		return "", errAuthUnavailable
	}
	return user.ID, nil
}
