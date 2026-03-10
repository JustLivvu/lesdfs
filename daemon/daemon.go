package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"os/signal"

	"lesd2/vaultfs"

	"bazil.org/fuse"
	bfs "bazil.org/fuse/fs"
)

const MountBaseDir = "/home/livvy/lesd_mount"

type VaultDaemon struct {
	mounts map[string]string
	mu     sync.Mutex
}

func NewVaultDaemon() *VaultDaemon {
	return &VaultDaemon{
		mounts: make(map[string]string),
	}
}

func (vd *VaultDaemon) MountVault(vaultName, vaultPath, password string) error {
	mountPath := filepath.Join(MountBaseDir, vaultName)
	if err := os.MkdirAll(MountBaseDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(mountPath, 0700); err != nil {
		return err
	}

	salt, err := vaultfs.InitSalt(vaultPath)
	if err != nil {
		return err
	}
	key := vaultfs.DeriveKey(password, salt)

	c, err := fuse.Mount(
		mountPath,
		fuse.FSName("lesd"),
		fuse.Subtype("lesdfs"),
		fuse.DefaultPermissions(),
	)
	if err != nil {
		return err
	}

	vd.mu.Lock()
	vd.mounts[vaultName] = mountPath
	vd.mu.Unlock()

	defer func() {
		fuse.Unmount(mountPath)
		vd.mu.Lock()
		delete(vd.mounts, vaultName)
		vd.mu.Unlock()
	}()

	filesys := &vaultfs.VaultFS{VaultPath: vaultPath, Key: key}

	if err := bfs.Serve(c, filesys); err != nil {
		log.Println("Serve error:", err)
	}

	return nil
}

func (vd *VaultDaemon) UnmountVault(vaultName string) {
	vd.mu.Lock()
	mountPath, ok := vd.mounts[vaultName]
	vd.mu.Unlock()
	if ok {
		fuse.Unmount(mountPath)
		vd.mu.Lock()
		delete(vd.mounts, vaultName)
		vd.mu.Unlock()
	}
}

func main() {
	vd := NewVaultDaemon()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("Unmounting all vaults...")
		vd.mu.Lock()
		for _, mountPath := range vd.mounts {
			fuse.Unmount(mountPath)
		}
		vd.mu.Unlock()
		os.Exit(0)
	}()

	http.HandleFunc("/mount", func(w http.ResponseWriter, r *http.Request) {
		vault := r.URL.Query().Get("vault")
		vaultPath := r.URL.Query().Get("path")
		password := r.URL.Query().Get("password")
		if vault == "" || vaultPath == "" || password == "" {
			w.WriteHeader(400)
			w.Write([]byte("missing parameters"))
			return
		}

		go func() {
			if err := vd.MountVault(vault, vaultPath, password); err != nil {
				log.Println("MountVault error:", err)
			}
		}()

		w.Write([]byte("ok"))
	})

	http.HandleFunc("/unmount", func(w http.ResponseWriter, r *http.Request) {
		vault := r.URL.Query().Get("vault")
		if vault == "" {
			w.WriteHeader(400)
			w.Write([]byte("missing vault name"))
			return
		}
		vd.UnmountVault(vault)
		w.Write([]byte("ok"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatal("cannot listen:", err)
	}

	log.Println("LES Vault Daemon v0.1 running on localhost:8080")
	log.Fatal(http.Serve(listener, nil))
}