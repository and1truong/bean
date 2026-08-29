package auth_test

import (
	"context"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/migration"
	"path/filepath"
	"testing"
)

func TestPasswordsAndSessions(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "auth.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.ExecuteMigration(ctx, migration.MetadataSchema()); e != nil {
		t.Fatal(e)
	}
	s := auth.Service{DB: db}
	if e = s.Bootstrap(ctx, "Admin@Example.Test", "test-password"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Login(ctx, "admin@example.test", "wrong-password"); e == nil {
		t.Fatal("wrong password accepted")
	}
	session, e := s.Login(ctx, "admin@example.test", "test-password")
	if e != nil {
		t.Fatal(e)
	}
	current, e := s.Current(ctx, session.ID)
	if e != nil || current.User.Email != "admin@example.test" {
		t.Fatalf("session=%+v err=%v", current, e)
	}
	if e = s.Logout(ctx, session.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Current(ctx, session.ID); e == nil {
		t.Fatal("logged out session accepted")
	}
}
