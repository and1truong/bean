package crashtest_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestReleasePublicationRecoversAfterProcessCrash(t *testing.T) {
	binary := os.Getenv("BEAN_BINARY")
	if binary == "" {
		t.Skip("set BEAN_BINARY to run black-box crash tests")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range []string{"release.after_migration", "release.after_activation_commit"} {
		t.Run(point, func(t *testing.T) {
			dir := t.TempDir()
			database := filepath.Join(dir, "bean.db")
			v1 := filepath.Join(dir, "v1.yaml")
			v2 := filepath.Join(dir, "v2.yaml")
			marker := filepath.Join(dir, "fault.marker")
			writeBundle(t, v1, false)
			writeBundle(t, v2, true)
			run(t, binary, nil, "app", "import", "--db", database, "--file", v1)
			run(t, binary, nil, "publish", "--db", database)
			run(t, binary, nil, "app", "import", "--db", database, "--file", v2)

			command := exec.Command(binary, "publish", "--db", database)
			command.Env = append(os.Environ(), "BEAN_FAULT_POINT="+point, "BEAN_FAULT_MARKER="+marker)
			if output, crashErr := command.CombinedOutput(); crashErr == nil {
				t.Fatalf("fault point did not terminate process: %s", output)
			}
			if contents, readErr := os.ReadFile(marker); readErr != nil || string(contents) != point {
				t.Fatalf("fault marker=%q err=%v", contents, readErr)
			}

			if point == "release.after_migration" {
				run(t, binary, nil, "publish", "--db", database)
			} else {
				run(t, binary, nil, "validate", "--db", database)
			}
			assertDatabase(t, database)
		})
	}
}

func writeBundle(t *testing.T, path string, author bool) {
	t.Helper()
	fields := "[{name: title, type: string, required: true}]"
	if author {
		fields = "[{name: title, type: string, required: true}, {name: author, type: string}]"
	}
	body := fmt.Sprintf("name: Crash test\ndefinitions:\n  - {apiVersion: bean/v1alpha1, kind: Entity, metadata: {name: book}, spec: {fields: %s}}\n", fields)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, binary string, environment []string, args ...string) {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), environment...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", binary, args, err, output)
	}
}

func assertDatabase(t *testing.T, path string) {
	t.Helper()
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var version int
	query := `SELECT r.version FROM bean_active_release a JOIN bean_release r ON r.id = a.release_id WHERE a.app_id = 'default'`
	if err = database.QueryRowContext(context.Background(), query).Scan(&version); err != nil || version != 2 {
		t.Fatalf("active version=%d err=%v", version, err)
	}
	var integrity string
	if err = database.QueryRowContext(context.Background(), "PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q err=%v", integrity, err)
	}
	var foreignKeyFailure int
	if err = database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyFailure); err != nil || foreignKeyFailure != 0 {
		t.Fatalf("foreign key failures=%d err=%v", foreignKeyFailure, err)
	}
	rows, err := database.QueryContext(context.Background(), `PRAGMA table_info("book")`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	foundAuthor := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		foundAuthor = foundAuthor || name == "author"
	}
	if !foundAuthor {
		t.Fatal("recovered release is missing book.author")
	}
}
