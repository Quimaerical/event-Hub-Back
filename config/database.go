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
			databaseURL = os.Getenv("POSTGRES_URL")
		}
		if databaseURL == "" {
			databaseURL = os.Getenv("POSTGRES_PRISMA_URL")
		}

		if databaseURL == "" {
			log.Println("ERROR: No se encontró la variable de entorno DATABASE_URL ni POSTGRES_URL")
			return
		}

		config, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			log.Printf("Error al analizar la cadena de conexión de la base de datos: %v", err)
			return
		}

		// Neon/Vercel Postgres Connection Pool Optimizations
		config.MaxConns = 8
		config.MinConns = 2
		config.MaxConnIdleTime = 15 * time.Minute
		config.MaxConnLifetime = 1 * time.Hour
		config.HealthCheckPeriod = 1 * time.Minute

		// Establish connection pool
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			log.Printf("Error estableciendo el pool de conexiones a la base de datos: %v", err)
			return
		}

		// Verify connection is active
		if err = pool.Ping(ctx); err != nil {
			log.Printf("Petición de Ping a la base de datos fallida: %v", err)
			return
		}

		DB = pool
		log.Println("Pool de conexiones a PostgreSQL establecido exitosamente")
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
