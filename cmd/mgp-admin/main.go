package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/techmigos/mgp/internal/auth"
	"github.com/techmigos/mgp/internal/config"
	"github.com/techmigos/mgp/internal/platform/database"
	"github.com/techmigos/mgp/internal/rbac"
)

func main() {
	username := flag.String("username", "", "account username")
	name := flag.String("name", "", "display name; required for a new account")
	pin := flag.String("pin", "", "six-digit PIN")
	role := flag.String("role", string(rbac.RoleAdmin), "ADMIN, INVENTORY, ISSUING, SECURITY, or VIEWER")
	flag.Parse()
	if *username == "" || *pin == "" {
		log.Fatal("-username and -pin are required")
	}
	if err := auth.ValidateUsername(*username); err != nil {
		log.Fatal(err)
	}
	parsedRole := rbac.Role(*role)
	if parsedRole != rbac.RoleAdmin && parsedRole != rbac.RoleInventory && parsedRole != rbac.RoleIssuing && parsedRole != rbac.RoleSecurity && parsedRole != rbac.RoleViewer {
		log.Fatal("invalid role")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(context.Background(), db); err != nil {
		log.Fatal(err)
	}
	hash, err := auth.HashPIN(*pin)
	if err != nil {
		log.Fatal(err)
	}
	store := auth.UserStore{DB: db}
	existing, findErr := store.FindByUsername(context.Background(), *username)
	if findErr == nil {
		if existing.Role != parsedRole {
			log.Fatalf("existing username %q belongs to role %s", *username, existing.Role)
		}
		if err := store.ResetPIN(context.Background(), existing.ID, hash); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stdout, "reset PIN for %s account %s (%s)\n", parsedRole, auth.NormalizeUsername(*username), existing.ID)
		return
	}
	if !errors.Is(findErr, sql.ErrNoRows) {
		log.Fatal(findErr)
	}
	if *name == "" {
		log.Fatal("-name is required when creating a new account")
	}
	id, err := store.Create(context.Background(), auth.User{Username: *username, Name: *name, PINHash: hash, Role: parsedRole})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "created %s account %s (%s)\n", parsedRole, auth.NormalizeUsername(*username), id)
}
