// Package main contains the core server application logic and components.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/pedrorobsonleao/golangapi/src/api"
)

var (
	// ErrNotFound is returned when a requested database resource does not exist.
	ErrNotFound = errors.New("resource not found")
)

// Store defines the database contract required by the application handlers.
// This interface allows for easy mocking in unit tests (e.g., MockStore).
type Store interface {
	// GetAll retrieves all registered individuals (pessoas) from the database.
	GetAll() ([]api.Pessoa, error)

	// GetById retrieves a specific individual by their unique ID.
	// Returns ErrNotFound if the record doesn't exist.
	GetById(id int64) (*api.Pessoa, error)

	// Create inserts a new individual with the provided name and returns the created entity with its database-generated ID.
	Create(nome string) (*api.Pessoa, error)

	// Update updates the name of an existing individual.
	// Returns ErrNotFound if the target ID does not exist in the database.
	Update(id int64, nome string) (*api.Pessoa, error)

	// Delete removes an individual by their unique ID.
	// Returns ErrNotFound if the target ID does not exist.
	Delete(id int64) error
}

// SQLStore is a concrete, thread-safe implementation of Store using a standard relational MySQL database.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore instantiates and returns a new SQLStore configured with the provided sql.DB pool pointer.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// GetAll implements the Store interface to query and scan all entries in the 'pessoa' table.
func (s *SQLStore) GetAll() ([]api.Pessoa, error) {
	rows, err := s.db.Query("SELECT id, nome FROM pessoa")
	if err != nil {
		return nil, fmt.Errorf("failed to query all pessoas: %w", err)
	}
	defer rows.Close()

	pessoas := []api.Pessoa{}
	for rows.Next() {
		var p api.Pessoa
		var id int64
		if err := rows.Scan(&id, &p.Nome); err != nil {
			return nil, fmt.Errorf("failed to scan pessoa: %w", err)
		}
		p.Id = &id
		pessoas = append(pessoas, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return pessoas, nil
}

// GetById queries a single row from the 'pessoa' table by its primary key.
// Correctly maps sql.ErrNoRows to ErrNotFound to keep database implementations decoupled from HTTP layers.
func (s *SQLStore) GetById(id int64) (*api.Pessoa, error) {
	var p api.Pessoa
	var dbId int64
	err := s.db.QueryRow("SELECT id, nome FROM pessoa WHERE id = ?", id).Scan(&dbId, &p.Nome)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query pessoa by id: %w", err)
	}
	p.Id = &dbId
	return &p, nil
}

// Create executes an INSERT statement in the 'pessoa' table and returns the primary key ID assigned by the database engine.
func (s *SQLStore) Create(nome string) (*api.Pessoa, error) {
	res, err := s.db.Exec("INSERT INTO pessoa (nome) VALUES (?)", nome)
	if err != nil {
		return nil, fmt.Errorf("failed to insert pessoa: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return &api.Pessoa{
		Id:   &id,
		Nome: nome,
	}, nil
}

// Update checks if the targeted individual exists before applying updates to provide exact and robust ErrNotFound status mapping.
func (s *SQLStore) Update(id int64, nome string) (*api.Pessoa, error) {
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM pessoa WHERE id = ?", id).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to check existence for update: %w", err)
	}

	_, err = s.db.Exec("UPDATE pessoa SET nome = ? WHERE id = ?", nome, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update pessoa: %w", err)
	}

	return &api.Pessoa{
		Id:   &id,
		Nome: nome,
	}, nil
}

// Delete checks if the targeted individual exists before running the DELETE query to ensure ErrNotFound is returned correctly.
func (s *SQLStore) Delete(id int64) error {
	var exists int
	err := s.db.QueryRow("SELECT 1 FROM pessoa WHERE id = ?", id).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to check existence for delete: %w", err)
	}

	_, err = s.db.Exec("DELETE FROM pessoa WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete pessoa: %w", err)
	}

	return nil
}
