package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"caddy-proxy-ui/internal/service"
	"caddy-proxy-ui/internal/store"
)

// runResetPassword reads a new password as a single line from stdin and
// updates the existing admin account, invalidating any active sessions.
// Deliberately dumb/scriptable — hiding the input from the terminal is left
// to the caller (e.g. contrib/install.sh uses `read -s`), rather than
// pulling in a terminal-handling dependency here.
func runResetPassword() {
	dbPath := envOr("DB_PATH", "./data/caddy-ui.db")

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	auth := service.NewAuthService(st)

	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		log.Fatalf("read password from stdin: %v", err)
	}
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		log.Fatal("password must be at least 8 characters")
	}

	if err := auth.ResetPassword(password); err != nil {
		log.Fatalf("reset password: %v", err)
	}
	fmt.Println("Password reset. Any existing dashboard sessions were signed out.")
}
