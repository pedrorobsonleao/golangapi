package main

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/pedrorobsonleao/golangapi/src/api"
)

// Server implements the generated api.ServerInterface.
// It manages HTTP request handlers, credentials, and coordinates security using RSA JWT signature signing.
type Server struct {
	store     Store
	signKey   *rsa.PrivateKey
	adminUser string
	adminPass string
}

// NewServer returns a fully configured Server instance.
// Fallback default admin credentials are "admin" / "admin" if empty strings are supplied.
func NewServer(store Store, signKey *rsa.PrivateKey, adminUser, adminPass string) *Server {
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "admin"
	}
	return &Server{
		store:     store,
		signKey:   signKey,
		adminUser: adminUser,
		adminPass: adminPass,
	}
}

// Login handles credentials validation (POST /login).
// Upon successful authentication, generates a signed RS256 JWT token using the RSA Private Key.
// The JWT token is valid for 24 hours.
func (s *Server) Login(ctx echo.Context) error {
	var req api.LoginRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	username := ""
	if req.Username != nil {
		username = *req.Username
	}
	password := ""
	if req.Password != nil {
		password = *req.Password
	}

	// Validate credentials against configured admin variables
	if username != s.adminUser || password != s.adminPass {
		return ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// Sign claim using RSA private key
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
		"iat": time.Now().Unix(),
	})

	tokenStr, err := token.SignedString(s.signKey)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
	}

	return ctx.JSON(http.StatusOK, map[string]string{"token": tokenStr})
}

// GetAll returns a list of all individuals (GET /pessoa).
func (s *Server) GetAll(ctx echo.Context) error {
	pessoas, err := s.store.GetAll()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, pessoas)
}

// Create inserts a new individual after validating character boundary constraints (POST /pessoa).
// Enforces a name size limit of 3 to 255 characters (as required by the Postman collection).
func (s *Server) Create(ctx echo.Context) error {
	var body api.Pessoa
	if err := ctx.Bind(&body); err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate input constraints
	if len(body.Nome) < 3 || len(body.Nome) > 255 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "nome deve ter entre 3 e 255 caracteres"})
	}

	created, err := s.store.Create(body.Nome)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return ctx.JSON(http.StatusOK, created)
}

// Delete removes an individual by their unique ID (DELETE /pessoa/{id}).
// Translates ErrNotFound directly to a 404 Status Not Found.
func (s *Server) Delete(ctx echo.Context, id int64) error {
	err := s.store.Delete(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ctx.JSON(http.StatusNotFound, map[string]string{"error": "pessoa nao encontrada"})
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "pessoa deletada com sucesso"})
}

// GetById retrieves a single individual by their ID (GET /pessoa/{id}).
func (s *Server) GetById(ctx echo.Context, id int64) error {
	p, err := s.store.GetById(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ctx.JSON(http.StatusNotFound, map[string]string{"error": "pessoa nao encontrada"})
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, p)
}

// Update modifies the name of an existing individual after validating boundaries (PUT /pessoa/{id}).
// Requires name length to be between 3 and 255 characters.
func (s *Server) Update(ctx echo.Context, id int64) error {
	var body api.Pessoa
	if err := ctx.Bind(&body); err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate input constraints
	if len(body.Nome) < 3 || len(body.Nome) > 255 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "nome deve ter entre 3 e 255 caracteres"})
	}

	updated, err := s.store.Update(id, body.Nome)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ctx.JSON(http.StatusNotFound, map[string]string{"error": "pessoa nao encontrada"})
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return ctx.JSON(http.StatusOK, updated)
}
