package testutil

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// DBIntegrationSuite is a testify suite that sets up a PostgreSQL container
// for integration tests.
type DBIntegrationSuite struct {
	suite.Suite
	Pool             *pgxpool.Pool
	pgContainer      *postgres.PostgresContainer
	ConnectionString string
}

// SetupSuite starts a PostgreSQL container before any tests in the suite are run.
func (s *DBIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// Find the project root to locate the schema.sql file
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../") // Assumes testutil is one level deep
	schemaPath := filepath.Join(projectRoot, "./sample/infra/postgres")

	dbName := "testdb"
	dbUser := "testuser"
	dbPassword := "testpassword"

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.WithInitScripts(filepath.Join(schemaPath, "schema.sql")),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("could not start postgres container: %s", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("could not get connection string: %s", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("could not connect to test database: %s", err)
	}

	s.Pool = pool
	s.pgContainer = container
	s.ConnectionString = connStr
}

// TearDownSuite stops and removes the container after all tests in the suite have been run.
func (s *DBIntegrationSuite) TearDownSuite() {
	if s.pgContainer != nil {
		if err := s.pgContainer.Terminate(context.Background()); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}
}

// TruncateTables is a helper to clean the database state between tests.
func (s *DBIntegrationSuite) TruncateTables(tables ...string) {
	for _, table := range tables {
		_, err := s.Pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY", table))
		s.Require().NoError(err, "failed to truncate table %s", table)
	}
}
