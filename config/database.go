package config

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	DB   *pgxpool.Pool
	once sync.Once
)

// ConnectDB initializes the database connection pool using pgxpool.
// It retrieves the DATABASE_URL environment variable and optimizes
// connection settings for resource-constrained serverless environments (Neon, Render).
func ConnectDB() *pgxpool.Pool {
	once.Do(func() {
		databaseURL := os.Getenv("DATABASE_URL")
		if databaseURL == "" {
			log.Fatal("DATABASE_URL environment variable is not set")
		}

		config, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			log.Fatalf("Error parsing DATABASE_URL: %v", err)
		}

		// Neon/Render Connection Pool Optimizations
		// Neon free tier supports up to 10-20 active connections.
		config.MaxConns = 8
		config.MinConns = 2
		config.MaxConnIdleTime = 15 * time.Minute
		config.MaxConnLifetime = 1 * time.Hour
		config.HealthCheckPeriod = 1 * time.Minute

		// Establish connection pool
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		DB, err = pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			log.Fatalf("Error establishing database connection pool: %v", err)
		}

		// Verify connection is active
		if err = DB.Ping(ctx); err != nil {
			log.Fatalf("Database ping failed: %v", err)
		}

		log.Println("Database connection pool successfully established")
	})

	return DB
}

// CloseDB closes the database connection pool.
func CloseDB() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection pool closed")
	}
}
