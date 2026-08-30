package postgres_test

import (
	"os"
	"testing"

	"github.com/beanruntime/bean/internal/dbal/dbaltest"
	"github.com/beanruntime/bean/internal/dbal/postgres"
)

func TestContract(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL contracts")
	}
	database, err := postgres.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dbaltest.Contract(t, database)
}

func TestCompilerUsesNumberedParameters(t *testing.T) {
	if got := (postgres.Compiler{}).Placeholder(3); got != "$3" {
		t.Fatalf("placeholder=%q", got)
	}
}
