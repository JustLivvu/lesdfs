package main

import (
	"lesd2/vaultfs"
	"errors"
	"fmt"
	"log"
	"io"
	"crypto/rand"
	"net/http"
	"os"
	"path/filepath"
)

const (
	BaseVaultDir = ".lesd"
	DaemonURL    = "http://127.0.0.1:8080"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: lesd --create|--list|--open|--delete <vault_name>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "--create":
		if len(os.Args) != 3 {
			log.Fatal("Usage: lesd --create <vault_name>")
		}
		if err := createVault(os.Args[2]); err != nil {
			log.Fatal(err)
		}
	case "--list":
		listVaults()
	case "--delete":
		if len(os.Args) != 3 {
			log.Fatal("Usage: lesd --delete <vault_name>")
		}
		if err := deleteVault(os.Args[2]); err != nil {
			log.Fatal(err)
		}
	case "--open":
		if len(os.Args) != 3 {
			log.Fatal("Usage: lesd --open <vault_name>")
		}
		if err := openVault(os.Args[2]); err != nil {
			log.Fatal(err)
		}

	case "--close":
		if len(os.Args) != 3 {
			log.Fatal("Usage: lesd --close <vault_name>")
		}
		if err := closeVault(os.Args[2]); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("Unknown command")
	}
}

func closeVault(name string) error {

	resp, err := http.Get(fmt.Sprintf("%s/unmount?vault=%s", DaemonURL, name))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("daemon failed to unmount vault")
	}

	fmt.Printf("Vault '%s' unmounted.\n", name)

	return nil
}

func getVaultPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, BaseVaultDir, name)
}

func createVault(name string) error {
	path := getVaultPath(name)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return errors.New("vault already exists")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	password, err := vaultfs.ReadPassword("Enter password: ")
	if err != nil {
		return err
	}

	salt := make([]byte, 16)
	if _, err := vaultfs.ReadPassword("Random salt: "); err != nil {
		return err
	}
	_, _ = io.ReadFull(rand.Reader, salt) 

	if err := os.WriteFile(filepath.Join(path, ".salt"), salt, 0600); err != nil {
		return err
	}

	fmt.Printf("Vault '%s' created.\n", name)
	_ = password
	return nil
}

func listVaults() {
	home, _ := os.UserHomeDir()
	vaultBase := filepath.Join(home, BaseVaultDir)
	files, err := os.ReadDir(vaultBase)
	if err != nil {
		fmt.Println("No vaults found.")
		return
	}
	for _, f := range files {
		if f.IsDir() {
			fmt.Println(f.Name())
		}
	}
}

func deleteVault(name string) error {
	return os.RemoveAll(getVaultPath(name))
}

func openVault(name string) error {
	vaultPath := getVaultPath(name)
	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		return errors.New("vault does not exist")
	}

	password, err := vaultfs.ReadPassword("Enter password: ")
	if err != nil {
		return err
	}

	resp, err := http.Get(fmt.Sprintf("%s/mount?vault=%s&path=%s&password=%s",
		DaemonURL, name, vaultPath, password))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("daemon failed to mount vault")
	}

	fmt.Printf("Vault '%s' mounted via daemon.\n", name)

	return nil
}