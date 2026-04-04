package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SessionClaims struct {
	Sub       string `json:"sub"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Exp       int64  `json:"exp"`
}

func CreateSessionToken(userID, login, name, email, avatarURL string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("userID is required")
	}

	headerRaw, err := json.Marshal(map[string]string{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("marshal token header: %w", err)
	}

	payloadRaw, err := json.Marshal(map[string]any{
		"sub":        userID,
		"login":      login,
		"name":       name,
		"email":      email,
		"avatar_url": avatarURL,
		"exp":        time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal token payload: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString(headerRaw)
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)

	return header + "." + payload + "." + strconv.FormatInt(time.Now().UnixNano(), 36), nil
}

func ParseSessionToken(token string) (*SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode token payload: %w", err)
	}

	var claims SessionClaims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal token payload: %w", err)
	}

	if claims.Sub == "" {
		return nil, fmt.Errorf("token subject is required")
	}

	if claims.Exp > 0 && time.Unix(claims.Exp, 0).Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}
