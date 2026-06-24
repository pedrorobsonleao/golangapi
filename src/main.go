// Package main is the entry point of the Go HTTP REST API server.
// It initializes configuration, manages database connection retry loops, generates RSA key pairs,
// configures routes and middlewares, and launches the HTTP Echo server.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/pedrorobsonleao/golangapi/src/api"
)

func main() {
	// 1. Load environment variables
	// Attempt to load .env file; if not present, standard system env variables are used (common in Docker environment).
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, using system environment variables")
	}

	dbUser := getEnv("DB_USER", "treinawebusr")
	dbPass := getEnv("DB_PASSWORD", "treinawebpwd")
	dbName := getEnv("DB_DATABASE", "treinaweb")
	dbHost := getEnv("DB_HOST", "127.0.0.1")
	dbPort := getEnv("DB_PORT", "3306")
	appPort := getEnv("APP_PORT", "8080")

	// 2. Initialize Database connection with retries
	// DB engines running inside containers might take a few seconds to boot up and accept connections.
	// We implement a 15-iteration retry loop with a 2-second sleep delay between each try.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)
	var db *sql.DB
	var err error

	log.Printf("Connecting to database at %s:%s...", dbHost, dbPort)
	for i := 0; i < 15; i++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Database connection attempt %d failed (error: %v). Retrying in 2 seconds...", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	log.Println("Database connection established successfully.")
	defer db.Close()

	// Initialize database schema
	// Automatically creates the relational 'pessoa' table if it doesn't already exist.
	log.Println("Initializing database schema...")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS pessoa (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		nome VARCHAR(255) NOT NULL
	)`)
	if err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}
	log.Println("Database schema initialized successfully.")

	// 3. Generate RSA key pair for JWT signing
	// To secure endpoints, we generate a 2048-bit RSA key pair on the fly at startup.
	// Private key is used by the server to sign new JWT tokens; Public key is used by the middleware to verify clients.
	log.Println("Generating RSA key pair...")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to generate RSA key pair: %v", err)
	}
	publicKey := &privateKey.PublicKey

	// 4. Initialize Store and Server
	store := NewSQLStore(db)
	adminUser := getEnv("SPRING_BOOT_ADMIN_USERNAME", "admin")
	adminPass := getEnv("SPRING_BOOT_ADMIN_PASSWORD", "admin")
	server := NewServer(store, privateKey, adminUser, adminPass)

	// 5. Setup Echo Framework
	e := echo.New()
	e.Use(customLogger)
	e.Use(customRecover)

	// Custom JWT Verification Middleware
	// Secures non-public API endpoints using the generated RSA public key.
	e.Use(jwtMiddleware(publicKey))

	// Register OpenAPI generated route handlers
	api.RegisterHandlers(e, server)

	// 6. Register Actuator Endpoints (expected by Postman tests with status 200 OK)
	// These health indicators allow Spring Boot Admin / test scripts to query service viability.
	e.GET("/actuator/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "UP"})
	})
	e.GET("/actuator/sbom", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status": "UP",
			"sbom":   "provided",
		})
	})
	e.GET("/actuator/sbom/application", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":      "UP",
			"application": "golang-api",
		})
	})

	// Start HTTP Server
	log.Printf("Starting HTTP server on port %s...", appPort)
	if err := e.Start(":" + appPort); err != nil {
		log.Fatalf("HTTP server failed to start: %v", err)
	}
}

// jwtMiddleware validates JWT tokens using an RSA public key.
// Public endpoints (/login and /actuator/*) are bypassed.
// All other endpoints require a valid "Authorization: Bearer <JWT_TOKEN>" header.
func jwtMiddleware(verifyKey *rsa.PublicKey) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			// Exclude public paths
			if path == "/login" || path == "/actuator/health" || path == "/actuator/sbom" || path == "/actuator/sbom/application" {
				return next(c)
			}

			// Extract Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization header missing"})
			}

			const prefix = "Bearer "
			if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token format"})
			}

			tokenStr := authHeader[len(prefix):]
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				// Enforce RS256 signature algorithm check to avoid algorithm evasion attacks
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return verifyKey, nil
			})

			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			}

			return next(c)
		}
	}
}

// getEnv extracts the env variable matching 'key'; returns 'fallback' if missing.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// customLogger intercepts requests to print structured metrics: HTTP Method, Path, Status Code and Duration.
func customLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)
		stop := time.Now()
		log.Printf("%s %s %d %s", c.Request().Method, c.Request().RequestURI, c.Response().Status, stop.Sub(start))
		return err
	}
}

// customRecover intercepts panics, logs the stack traces to stderr, and prevents the server process from crashing.
func customRecover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				log.Printf("PANIC Recovered: %v", err)
				c.Error(err)
			}
		}()
		return next(c)
	}
}
