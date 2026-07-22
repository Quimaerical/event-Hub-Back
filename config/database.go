package config

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

var (
	DB   *pgxpool.Pool
	once sync.Once
)

// ConnectDB initializes the database connection pool using pgxpool.
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
		config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
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

		// Intentar Ping inicial
		if err = pool.Ping(ctx); err != nil {
			log.Printf("Aviso: El ping inicial a la BD falló (posible reactivación de Neon/Postgres): %v", err)
		} else {
			log.Println("Pool de conexiones a PostgreSQL establecido exitosamente")
		}

		// Asignamos el pool a DB
		DB = pool

		// IMPORTANTE EN SERVERLESS: Ejecutar auto-migración sincrónica (sin goroutine)
		// para evitar congelamiento de la CPU por parte de Vercel/Lambda
		autoCtx, autoCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer autoCancel()
		if err := AutoMigrateIfEmpty(autoCtx); err != nil {
			log.Printf("Aviso en AutoMigrateIfEmpty: %v", err)
		}
	})

	return DB
}

// AutoMigrateIfEmpty comprueba si la base de datos está vacía y ejecuta schema.sql automáticamente
func AutoMigrateIfEmpty(ctx context.Context) error {
	if DB == nil {
		return errors.New("base de datos no conectada")
	}

	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'roles')`
	err := DB.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		log.Printf("Aviso al verificar existencia de tabla 'roles': %v", err)
	}

	if !exists {
		log.Println("Base de datos en Neon sin tablas detectada. Ejecutando auto-migración de schema.sql sincrónicamente...")
		_, err := DB.Exec(ctx, schemaSQL)
		if err != nil {
			log.Printf("Error ejecutando auto-migración de schema.sql: %v", err)
			return err
		}
		log.Println("¡Auto-migración completada exitosamente en Neon PostgreSQL!")
	} else {
		log.Println("Base de datos verificada: Las tablas principales existen.")
	}

	return nil
}

// ForceMigrate ejecuta explícitamente el script schema.sql para recrear o reiniciar las tablas
func ForceMigrate(ctx context.Context) error {
	if DB == nil {
		return errors.New("base de datos no conectada")
	}
	log.Println("Ejecutando migración forzada de schema.sql en Neon PostgreSQL...")
	_, err := DB.Exec(ctx, schemaSQL)
	if err != nil {
		return err
	}
	DB.Reset()
	return nil
}

// CloseDB closes the database connection pool.
func CloseDB() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection pool closed")
	}
}
