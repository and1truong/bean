package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
)

func TestHostPasswordRecoveryFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.db")
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err := runtime.HTTP.Auth.Create(ctx, "member@example.test", "old-password", []string{"authenticated"}, ""); err != nil {
		t.Fatal(err)
	}
	session, err := runtime.HTTP.Auth.Login(ctx, "member@example.test", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--db", path, "--email", " MEMBER@EXAMPLE.TEST ", "--password-stdin"}
	if err := resetPasswordCommand(args, strings.NewReader("new-password\r\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err == nil {
		t.Fatal("recovery kept old session")
	}
	if _, err := runtime.HTTP.Auth.Login(ctx, "member@example.test", "old-password"); err == nil {
		t.Fatal("old password accepted")
	}
	if _, err := runtime.HTTP.Auth.Login(ctx, "member@example.test", "new-password"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"short", strings.Repeat("a", 73), strings.Repeat("é", 37)} {
		if err := resetPasswordCommand(args, strings.NewReader(input)); err == nil {
			t.Fatal("invalid password accepted")
		}
	}
	if err := resetPasswordCommand([]string{"--db", path, "--email", "member@example.test"}, strings.NewReader("new-password")); err == nil {
		t.Fatal("stdin not explicitly enabled")
	}
	if err := resetPasswordCommand([]string{"--db", path, "--email", "missing@example.test", "--password-stdin"}, strings.NewReader("new-password")); err == nil {
		t.Fatal("recovery created missing account")
	}
}
