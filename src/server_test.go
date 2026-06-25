package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/pedrorobsonleao/golangapi/src/api"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	Pessoas []api.Pessoa
	NextId  int64
	Err     error
}

func (m *MockStore) GetAll() ([]api.Pessoa, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Pessoas, nil
}

func (m *MockStore) GetById(id int64) (*api.Pessoa, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	for _, p := range m.Pessoas {
		if p.Id != nil && *p.Id == id {
			return &p, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) Create(nome string) (*api.Pessoa, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	id := m.NextId
	m.NextId++
	p := api.Pessoa{Id: &id, Nome: nome}
	m.Pessoas = append(m.Pessoas, p)
	return &p, nil
}

func (m *MockStore) Update(id int64, nome string) (*api.Pessoa, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	for i, p := range m.Pessoas {
		if p.Id != nil && *p.Id == id {
			m.Pessoas[i].Nome = nome
			return &m.Pessoas[i], nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockStore) Delete(id int64) error {
	if m.Err != nil {
		return m.Err
	}
	for i, p := range m.Pessoas {
		if p.Id != nil && *p.Id == id {
			m.Pessoas = append(m.Pessoas[:i], m.Pessoas[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func setupTestServer() (*Server, *MockStore, *rsa.PrivateKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	mockStore := &MockStore{
		Pessoas: []api.Pessoa{},
		NextId:  1,
	}
	server := NewServer(mockStore, privateKey, "admin", "admin")
	return server, mockStore, privateKey
}

func TestLogin(t *testing.T) {
	server, _, _ := setupTestServer()
	e := echo.New()

	// 1. Success Login
	reqBody := `{"username":"admin","password":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.Login(c); err != nil {
		t.Fatalf("Login handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := resp["token"]; !ok {
		t.Errorf("Expected token in response, got %v", resp)
	}

	// 2. Invalid Credentials
	reqBodyWrong := `{"username":"admin","password":"wrong"}`
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(reqBodyWrong))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.Login(c); err != nil {
		t.Fatalf("Login handler failed: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %d", rec.Code)
	}
}

func TestCreatePessoa(t *testing.T) {
	server, mockStore, _ := setupTestServer()
	e := echo.New()

	// 1. Successful creation
	reqBody := `{"nome":"João da Silva"}`
	req := httptest.NewRequest(http.MethodPost, "/pessoa", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.Create(c); err != nil {
		t.Fatalf("Create handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	var p api.Pessoa
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if p.Nome != "João da Silva" || p.Id == nil || *p.Id != 1 {
		t.Errorf("Unexpected created pessoa: %+v", p)
	}

	// 2. Name too short (validation check)
	reqBodyShort := `{"nome":"Ab"}`
	req = httptest.NewRequest(http.MethodPost, "/pessoa", strings.NewReader(reqBodyShort))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.Create(c); err != nil {
		t.Fatalf("Create handler failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest (validation) for short name, got %d", rec.Code)
	}

	// 3. Name too long (validation check)
	longName := strings.Repeat("A", 256)
	reqBodyLong := `{"nome":"` + longName + `"}`
	req = httptest.NewRequest(http.MethodPost, "/pessoa", strings.NewReader(reqBodyLong))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.Create(c); err != nil {
		t.Fatalf("Create handler failed: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest (validation) for long name, got %d", rec.Code)
	}

	// Validate mock store size
	if len(mockStore.Pessoas) != 1 {
		t.Errorf("Expected mock store to contain 1 record, got %d", len(mockStore.Pessoas))
	}
}

func TestGetAllPessoas(t *testing.T) {
	server, mockStore, _ := setupTestServer()
	e := echo.New()

	// Seed records
	mockStore.Create("Ana")
	mockStore.Create("Bruno")

	req := httptest.NewRequest(http.MethodGet, "/pessoa", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.GetAll(c); err != nil {
		t.Fatalf("GetAll handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	var list []api.Pessoa
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 records, got %d", len(list))
	}
}

func TestGetPessoaById(t *testing.T) {
	server, mockStore, _ := setupTestServer()
	e := echo.New()

	// Seed records
	mockStore.Create("Carlos")

	// 1. Success get
	req := httptest.NewRequest(http.MethodGet, "/pessoa/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.GetById(c, 1); err != nil {
		t.Fatalf("GetById handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	var p api.Pessoa
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if p.Nome != "Carlos" {
		t.Errorf("Expected Carlos, got %s", p.Nome)
	}

	// 2. Not Found get
	req = httptest.NewRequest(http.MethodGet, "/pessoa/999", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.GetById(c, 999); err != nil {
		t.Fatalf("GetById handler failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status NotFound, got %d", rec.Code)
	}
}

func TestUpdatePessoa(t *testing.T) {
	server, mockStore, _ := setupTestServer()
	e := echo.New()

	mockStore.Create("Daniela")

	// 1. Success update
	reqBody := `{"nome":"Daniela Silva"}`
	req := httptest.NewRequest(http.MethodPut, "/pessoa/1", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.Update(c, 1); err != nil {
		t.Fatalf("Update handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	var p api.Pessoa
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if p.Nome != "Daniela Silva" {
		t.Errorf("Expected Daniela Silva, got %s", p.Nome)
	}

	// 2. Not Found update
	reqBodyNotFound := `{"nome":"Daniela Silva"}`
	req = httptest.NewRequest(http.MethodPut, "/pessoa/999", strings.NewReader(reqBodyNotFound))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.Update(c, 999); err != nil {
		t.Fatalf("Update handler failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status NotFound, got %d", rec.Code)
	}
}

func TestDeletePessoa(t *testing.T) {
	server, mockStore, _ := setupTestServer()
	e := echo.New()

	mockStore.Create("Eduardo")

	// 1. Success delete
	req := httptest.NewRequest(http.MethodDelete, "/pessoa/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.Delete(c, 1); err != nil {
		t.Fatalf("Delete handler failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}

	if len(mockStore.Pessoas) != 0 {
		t.Errorf("Expected store to be empty, got %d", len(mockStore.Pessoas))
	}

	// 2. Not Found delete
	req = httptest.NewRequest(http.MethodDelete, "/pessoa/999", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	if err := server.Delete(c, 999); err != nil {
		t.Fatalf("Delete handler failed: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status NotFound, got %d", rec.Code)
	}
}

func TestSwaggerUI(t *testing.T) {
	e := echo.New()

	e.GET("/swagger-ui/openapi.yaml", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "text/yaml; charset=utf-8", openAPISpec)
	})

	e.GET("/swagger-ui", func(c echo.Context) error {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Swagger UI</title>
</head>
<body>
  <div id="swagger-ui"></div>
</body>
</html>`
		return c.HTML(http.StatusOK, html)
	})

	e.GET("/swagger-ui/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger-ui")
	})

	// 1. Test /swagger-ui
	req := httptest.NewRequest(http.MethodGet, "/swagger-ui", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK for /swagger-ui, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Swagger UI") {
		t.Errorf("Expected body to contain 'Swagger UI'")
	}

	// 2. Test /swagger-ui/ (redirect)
	req = httptest.NewRequest(http.MethodGet, "/swagger-ui/", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Errorf("Expected status MovedPermanently for /swagger-ui/, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/swagger-ui" {
		t.Errorf("Expected Location header to be /swagger-ui, got %s", rec.Header().Get("Location"))
	}

	// 3. Test /swagger-ui/openapi.yaml
	req = httptest.NewRequest(http.MethodGet, "/swagger-ui/openapi.yaml", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK for /swagger-ui/openapi.yaml, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/yaml; charset=utf-8" {
		t.Errorf("Expected Content-Type text/yaml; charset=utf-8, got %s", rec.Header().Get("Content-Type"))
	}
}
