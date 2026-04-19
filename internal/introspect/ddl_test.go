package introspect

import (
	"os"
	"path/filepath"
	"testing"
)

const testDDL = `
CREATE TYPE public.order_status AS ENUM ('pending', 'paid', 'shipped');

CREATE TABLE public.users (
    id         serial       PRIMARY KEY,
    email      text         NOT NULL,
    created_at timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE public.orders (
    id         bigserial    PRIMARY KEY,
    user_id    integer      NOT NULL,
    status     order_status NOT NULL DEFAULT 'pending',
    total      numeric(12,2),
    CONSTRAINT orders_user_fk FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
);

CREATE TABLE public.tags (
    id   serial PRIMARY KEY,
    name text   NOT NULL,
    UNIQUE (name)
);

ALTER TABLE public.orders ADD CONSTRAINT orders_unique_id UNIQUE (id);
`

func TestDDLIntrospect(t *testing.T) {
	f := filepath.Join(t.TempDir(), "schema.sql")
	if err := os.WriteFile(f, []byte(testDDL), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := (&DDL{}).Introspect(f, "public")
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}

	// Enums
	if len(m.Enums) != 1 {
		t.Fatalf("want 1 enum, got %d", len(m.Enums))
	}
	e := m.Enums[0]
	if e.Name != "order_status" || len(e.Values) != 3 {
		t.Errorf("enum mismatch: %+v", e)
	}

	// Tables
	if len(m.Tables) != 3 {
		t.Fatalf("want 3 tables, got %d", len(m.Tables))
	}

	users := m.FindTable("public", "users")
	if users == nil {
		t.Fatal("users table not found")
	}
	if len(users.PrimaryKey) != 1 || users.PrimaryKey[0] != "id" {
		t.Errorf("users PK: %v", users.PrimaryKey)
	}
	if len(users.Columns) != 3 {
		t.Errorf("users columns: %d", len(users.Columns))
	}

	// email NOT NULL
	var emailCol, idCol *struct{ nullable bool }
	for _, c := range users.Columns {
		switch c.Name {
		case "email":
			if c.Nullable {
				t.Error("email should be NOT NULL")
			}
		case "id":
			if c.DataType != "integer" {
				t.Errorf("id data_type: %s", c.DataType)
			}
		}
	}
	_ = emailCol
	_ = idCol

	orders := m.FindTable("public", "orders")
	if orders == nil {
		t.Fatal("orders table not found")
	}
	if len(orders.ForeignKeys) != 1 {
		t.Fatalf("orders FK count: %d", len(orders.ForeignKeys))
	}
	fk := orders.ForeignKeys[0]
	if fk.RefTable != "users" || fk.OnDelete != "CASCADE" {
		t.Errorf("orders FK: %+v", fk)
	}

	// status column links to enum
	for _, c := range orders.Columns {
		if c.Name == "status" && c.EnumName != "order_status" {
			t.Errorf("status EnumName: %q", c.EnumName)
		}
	}

	tags := m.FindTable("public", "tags")
	if tags == nil {
		t.Fatal("tags table not found")
	}
	if len(tags.UniqueKeys) != 1 || tags.UniqueKeys[0][0] != "name" {
		t.Errorf("tags unique keys: %v", tags.UniqueKeys)
	}
}
