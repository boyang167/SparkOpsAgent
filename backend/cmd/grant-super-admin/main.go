package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	identitycommand "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	identityquery "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain"
	identitypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func main() {
	email := flag.String("email", "", "member email (required)")
	password := flag.String("password", "", "password when registering a new member")
	displayName := flag.String("display-name", "", "display name when registering (defaults to email local-part)")
	register := flag.Bool("register", false, "register the member if email does not exist")
	flag.Parse()

	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/grant-super-admin -email user@example.com [-register] [-password ...] [-display-name ...]")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if *register && len(*password) < 8 {
		log.Fatal("-password is required (min 8 chars) when -register is set")
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Database.DSN == "" {
		log.Fatal("DATABASE_DSN or config.yaml database.dsn is required")
	}

	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	memberRepo := identitypersistence.NewMemberRepository(db)
	systemRoleStore := identitypersistence.NewSystemRoleStore(db)
	if err := systemRoleStore.AutoMigrate(); err != nil {
		log.Fatal(err)
	}
	systemRoles := identityquery.NewSystemRoleChecker(systemRoleStore)

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	member, err := memberRepo.FindByEmail(ctx, normalizedEmail)
	if err != nil {
		if de, ok := kernel.IsDomainError(err); ok && de.Code() == domain.ErrMemberNotFound.Code() {
			if !*register {
				log.Fatalf("member %q not found; pass -register to create", normalizedEmail)
			}
			name := strings.TrimSpace(*displayName)
			if name == "" {
				name = strings.Split(normalizedEmail, "@")[0]
			}
			registerHandler := identitycommand.NewRegisterMemberHandler(memberRepo, eventbus.NewInMemory())
			res, regErr := registerHandler.Handle(ctx, identitycommand.RegisterMemberInput{
				Email: normalizedEmail, Password: *password, DisplayName: name,
			})
			if regErr != nil {
				log.Fatal(regErr)
			}
			member, err = memberRepo.FindByID(ctx, res.MemberID)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("registered member: %s (%s)\n", member.Email, member.ID.String())
		} else {
			log.Fatal(err)
		}
	}

	if err := systemRoles.GrantSuperAdmin(ctx, member.ID); err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Super Admin granted ===")
	fmt.Printf("Email:     %s\n", member.Email)
	fmt.Printf("Member ID: %s\n", member.ID.String())
	fmt.Printf("Role:      super_admin\n")
	if *register {
		fmt.Printf("Password:  %s\n", *password)
	}
	fmt.Println()
	fmt.Println("Re-login or call GET /api/v1/auth/me to refresh is_super_admin.")
}
