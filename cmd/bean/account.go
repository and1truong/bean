package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/bootstrap"
)

func resetPasswordCommand(args []string, input io.Reader) error {
	flags := flag.NewFlagSet("user reset-password", flag.ContinueOnError)
	db := flags.String("db", "bean.db", "SQLite database")
	databaseURL := flags.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	email := flags.String("email", "", "existing user email")
	stdin := flags.Bool("password-stdin", false, "read the new password from stdin (never command arguments)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *email == "" || !*stdin || flags.NArg() != 0 {
		return fmt.Errorf("usage: bean user reset-password --email EMAIL --password-stdin [--db PATH]")
	}
	// bcrypt accepts at most 72 bytes; allow one optional LF or CRLF, not an
	// unbounded input stream. Spaces in passwords are deliberately preserved.
	raw, err := io.ReadAll(io.LimitReader(input, 75))
	if err != nil {
		return fmt.Errorf("cannot read password from stdin")
	}
	password := string(raw)
	if strings.HasSuffix(password, "\n") {
		password = strings.TrimSuffix(strings.TrimSuffix(password, "\n"), "\r")
	}
	if err := auth.ValidateNewPassword(password, password); err != nil {
		return err
	}
	runtime, err := bootstrap.OpenURL(context.Background(), databaseTarget(*db, *databaseURL), false)
	if err != nil {
		return err
	}
	defer runtime.DB.Close()
	return runtime.HTTP.Actions.ResetAccountPassword(context.Background(), *email, password)
}
