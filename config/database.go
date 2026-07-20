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
// connection settings for resource-constrained serverless environments (Neon, Render, Vercel).
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
			databaseURL = os.Getenv("POSTGRES_URL_NON_POOLING")
		}

		if databaseURL == "" {
			log.Println("ERROR CRÍTICO: No se encontró ninguna variable de entorno de base de datos (DATABASE_URL, POSTGRES_URL, etc.)")
			return
		}

		config, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			log.Printf("Error al analizar la cadena de conexión de la base de datos: %v", err)
			return
		}

		// Optimizaciones para Vercel Serverless & Neon PostgreSQL
		config.MaxConns = 10
		config.MinConns = 0 // CRÍTICO EN SERVERLESS: Debe ser 0 para evitar bloqueos en el arranque del contenedor
		config.MaxConnIdleTime = 5 * time.Minute
		config.MaxConnLifetime = 30 * time.Minute
		config.HealthCheckPeriod = 1 * time.Minute

		// Establish connection pool
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			log.Printf("Error estableciendo el pool de conexiones a la base de datos: %v", err)
			return
		}

		// Intentar Ping inicial (no bloqueante para la asignación del pool)
		if err = pool.Ping(ctx); err != nil {
			log.Printf("Aviso: El ping inicial a la BD falló (posible reactivación de Neon/Postgres): %v", err)
		} else {
			log.Println("Pool de conexiones a PostgreSQL establecido exitosamente")
		}

		// Asignamos el pool a DB para permitir reintentos en consultas posteriores
		DB = pool
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
