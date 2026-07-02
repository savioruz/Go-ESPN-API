package jwt

//go:generate go run go.uber.org/mock/mockgen -source=./jwt.go -destination=./mocks/jwt_mock.go -package=mocks

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go-espn-api/config"
	"go-espn-api/shared/errkey"
	"math/big"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksEndpoint = "/api/auth/jwks"
	httpTimeout  = 10 * time.Second
)

var (
	ErrJWKSFetchFailed    = errors.New(string(errkey.ErrJWKSFetchFailed))
	ErrNoMatchingKey      = errors.New("auth.no_matching_key")
	ErrUnsupportedKeyType = errors.New("auth.unsupported_key_type")
	ErrUnexpectedSigning  = errors.New(string(errkey.ErrTokenInvalid))
	ErrExpiredToken       = errors.New(string(errkey.ErrTokenExpired))
	ErrInvalidToken       = errors.New(string(errkey.ErrTokenInvalid))
	ErrInvalidClaim       = errors.New(string(errkey.ErrInvalidClaim))
	ErrAuthHeaderMissing  = errors.New(string(errkey.ErrAuthHeaderMissing))
	ErrInvalidAuthHeader  = errors.New(string(errkey.ErrInvalidAuthHeader))
	ErrTokenParseFailed   = errors.New("auth.token_parse_failed")
	ErrTokenMissingKid    = errors.New("auth.token_missing_kid")
	ErrTokenInvalidClaims = errors.New("auth.token_invalid_claims")
)

type Claims struct {
	Email   string `json:"email"`
	Role    string `json:"role"`
	Session string `json:"sid"`
	jwt.RegisteredClaims
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Crv string `json:"crv"`
}

type JWT interface {
	ValidateToken(ctx context.Context, tokenString string) (*Claims, error)
}

type Service struct {
	config   *config.Config
	jwks     *JWKS
	jwksOnce sync.Once
	jwksErr  error
}

func New(cfg *config.Config) JWT {
	return &Service{
		config: cfg,
	}
}

func (s *Service) fetchJWKS(ctx context.Context) error {
	u, _ := url.Parse(s.config.AuthService.URL)
	u.Path = path.Join(u.Path, jwksEndpoint)
	jwksURL := u.String()

	client := &http.Client{Timeout: httpTimeout}

	resp, err := client.Get(jwksURL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrJWKSFetchFailed, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrJWKSFetchFailed, resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("%w: %w", ErrJWKSFetchFailed, err)
	}

	s.jwks = &jwks

	return nil
}

func (s *Service) GetJWKS(ctx context.Context) (*JWKS, error) {
	s.jwksOnce.Do(func() {
		s.jwksErr = s.fetchJWKS(ctx)
	})

	if s.jwksErr != nil {
		return nil, s.jwksErr
	}

	return s.jwks, nil
}

func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*Claims, error) {
	jwks, err := s.GetJWKS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, ErrTokenParseFailed
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, ErrTokenMissingKid
	}

	var publicKey crypto.PublicKey

	for _, key := range jwks.Keys {
		if key.Kid == kid {
			switch key.Kty {
			case "EC", "OKP":
				publicKey, err = key.toECPublicKey()
				if err != nil {
					return nil, fmt.Errorf("failed to parse EC/OKP key: %w", err)
				}
			case "RSA":
				publicKey, err = key.toRSAPublicKey()
				if err != nil {
					return nil, fmt.Errorf("failed to parse RSA key: %w", err)
				}
			default:
				return nil, fmt.Errorf("%w: %s", ErrUnsupportedKeyType, key.Kty)
			}

			break
		}
	}

	if publicKey == nil {
		return nil, ErrNoMatchingKey
	}

	token, err = jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, ErrUnexpectedSigning
			}
		}

		return publicKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}

		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Issuer != s.config.AuthService.URL {
		return nil, ErrInvalidClaim
	}

	validAudience := false

	for _, aud := range claims.Audience {
		if aud == s.config.App.Name {
			validAudience = true

			break
		}
	}

	if !validAudience {
		return nil, ErrInvalidClaim
	}

	return claims, nil
}

func (k *JWK) toECPublicKey() (crypto.PublicKey, error) {
	if k.Crv == "Ed25519" && k.X != "" {
		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}

		publicKey := ed25519.PublicKey(xBytes)

		return publicKey, nil
	}

	// Handle EC keys (P-521 curve)
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, err
	}

	curve := elliptic.P521()
	key := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}

	return key, nil
}

func (k *JWK) toRSAPublicKey() (crypto.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{N: n, E: e}, nil
}

func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", ErrAuthHeaderMissing
	}

	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		return "", ErrInvalidAuthHeader
	}

	return authHeader[len(prefix):], nil
}
