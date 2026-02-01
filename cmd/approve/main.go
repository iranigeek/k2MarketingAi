package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"k2MarketingAi/internal/config"
	"k2MarketingAi/internal/storage"
)

func main() {
	var (
		configPath = flag.String("config", "config.json", "Path to config file")
		email      = flag.String("email", "", "User email to approve/disable")
		approved   = flag.Bool("approved", true, "Set approved state (true/false)")
		list       = flag.Bool("list", false, "List users")
		deleteFlag = flag.Bool("delete", false, "Delete user by email")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("database_url is required in config to approve users")
	}

	ctx := context.Background()
	store, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect store: %v", err)
	}
	defer store.Close()

	if *list {
		if err := listUsers(ctx, store); err != nil {
			log.Fatalf("list users: %v", err)
		}
		return
	}

	if *email == "" {
		log.Fatal("email is required (use -email)")
	}

	user, err := store.GetUserByEmail(ctx, *email)
	if err != nil {
		log.Fatalf("find user: %v", err)
	}

	if *deleteFlag {
		if err := store.DeleteUser(ctx, user.ID); err != nil {
			log.Fatalf("delete user: %v", err)
		}
		fmt.Printf("User %s (%s) deleted\n", user.Email, user.ID)
		return
	}

	if err := store.ApproveUser(ctx, user.ID, *approved); err != nil {
		log.Fatalf("update user: %v", err)
	}

	fmt.Printf("User %s (%s) approved=%v\n", user.Email, user.ID, *approved)
}

func listUsers(ctx context.Context, store storage.Store) error {
	users, err := store.ListUsers(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%-40s %-6s %-30s\n", "ID", "OK?", "EMAIL")
	for _, u := range users {
		fmt.Printf("%-40s %-6v %-30s\n", u.ID, u.Approved, u.Email)
	}
	return nil
}
