package postgres

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestHint(t *testing.T) {
	// error จริงจาก pgx ห่อหลายชั้น — Hint ต้องแกะเจอ
	wrap := func(err error) error { return fmt.Errorf("connect postgres@127.0.0.1:5432/postgres: %w", err) }

	tests := map[string]struct {
		err  error
		want string
	}{
		"wrong password":   {wrap(&pgconn.PgError{Code: "28P01", Message: "password authentication failed"}), "รหัสผ่าน"},
		"wrong database":   {wrap(&pgconn.PgError{Code: "3D000", Message: `database "map-maker-db-new" does not exist`}), "DB_SCHEMA"},
		"render needs ssl": {wrap(&pgconn.PgError{Code: "28000", Message: "SSL/TLS required"}), "sslmode=require"},
		"schema missing":   {fmt.Errorf("%w: x", ErrSchemaNotFound), "DB_SCHEMA"},
		"special chars":    {errors.New("parse DATABASE_URL: cannot parse `postgres://u:xxxxx@h/d`: invalid port"), "%40"},
		"local no ssl":     {wrap(errors.New("tls error: server refused TLS connection")), "sslmode=disable"},
		"not running":      {wrap(errors.New("dial tcp 127.0.0.1:5432: connectex: No connection could be made because the target machine actively refused it.")), "5432"},
		"typo in host":     {wrap(errors.New("hostname resolving error: lookup dpg-x.render.com: no such host")), "host"},
		"unknown":          {errors.New("something else"), ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Hint(tt.err)
			if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
				t.Fatalf("Hint = %q, want it to mention %q", got, tt.want)
			}
		})
	}
}

func TestDescribeHidesPassword(t *testing.T) {
	got := Describe("postgres://map_maker_api:s3cret@127.0.0.1:5432/postgres?sslmode=disable")

	if strings.Contains(got, "s3cret") {
		t.Fatalf("Describe leaked the password: %q", got)
	}
	if want := "map_maker_api@127.0.0.1:5432/postgres"; got != want {
		t.Fatalf("Describe = %q, want %q", got, want)
	}
}
