package main

import (
	"context"
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
	email := flag.String("email", "", "account email")
	name := flag.String("name", "", "display name")
	password := flag.String("password", "", "temporary password; change it before operational use")
	role := flag.String("role", string(rbac.RoleAdmin), "ADMIN, INVENTORY, ISSUING, SECURITY, or VIEWER")
	flag.Parse()
	if *email == "" || *name == "" || *password == "" {
		log.Fatal("-email, -name, and -password are required")
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
	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatal(err)
	}
	id, err := (auth.UserStore{DB: db}).Create(context.Background(), auth.User{Email: *email, Name: *name, Password: hash, Role: parsedRole})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "created %s account %s (%s)\n", parsedRole, *email, id)
}
