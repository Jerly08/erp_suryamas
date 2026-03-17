package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "gagal menentukan lokasi wrapper script")
		os.Exit(1)
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	backendDir := filepath.Join(repoRoot, "backend")
	targetScript := filepath.Join(backendDir, "scripts", "maintenance", "reset_transaction_data_gorm.go")

	if _, err := os.Stat(targetScript); err != nil {
		fmt.Fprintf(os.Stderr, "script target tidak ditemukan: %s\n", targetScript)
		os.Exit(1)
	}

	args := append([]string{"run", filepath.Join("scripts", "maintenance", "reset_transaction_data_gorm.go")}, os.Args[1:]...)
	cmd := exec.Command("go", args...)
	cmd.Dir = backendDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}

		fmt.Fprintf(os.Stderr, "gagal menjalankan script reset backend: %v\n", err)
		os.Exit(1)
	}
}
