package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"fmt"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

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
	log.Println("Initializing VaultDaemon")
	return &VaultDaemon{
		mounts: make(map[string]string),
	}
}

func (vd *VaultDaemon) MountVault(vaultName, vaultPath, password string) error {

	start := time.Now()
	log.Printf("MountVault requested: vault=%s path=%s", vaultName, vaultPath)

	mountPath := filepath.Join(MountBaseDir, vaultName)

	if err := os.MkdirAll(MountBaseDir, 0700); err != nil {
		log.Printf("ERROR creating base mount dir: %v", err)
		return err
	}

	if err := os.MkdirAll(mountPath, 0700); err != nil {
		log.Printf("ERROR creating mount dir %s: %v", mountPath, err)
		return err
	}

	log.Printf("Initializing vault salt: %s", vaultPath)

	salt, err := vaultfs.InitSalt(vaultPath)
	if err != nil {
		log.Printf("ERROR InitSalt: %v", err)
		return err
	}

	log.Printf("Deriving encryption key for vault=%s", vaultName)

	key := vaultfs.DeriveKey(password, salt)

	log.Printf("Mounting FUSE filesystem at %s", mountPath)

	c, err := fuse.Mount(
		mountPath,
		fuse.FSName("lesd"),
		fuse.Subtype("lesdfs"),
		fuse.DefaultPermissions(),
	)

	if err != nil {
		log.Printf("ERROR fuse.Mount failed: %v", err)
		return err
	}

	vd.mu.Lock()
	vd.mounts[vaultName] = mountPath
	vd.mu.Unlock()

	log.Printf("Vault mounted: %s -> %s", vaultName, mountPath)

	defer func() {

		log.Printf("FUSE session ended for vault=%s", vaultName)

		err := fuse.Unmount(mountPath)
		if err != nil {
			log.Printf("WARNING fuse.Unmount failed: %v", err)
		}

		vd.mu.Lock()
		delete(vd.mounts, vaultName)
		vd.mu.Unlock()

		log.Printf("Vault removed from active list: %s", vaultName)
	}()

	filesys := &vaultfs.VaultFS{
		VaultPath: vaultPath,
		Key:       key,
	}

	log.Printf("Starting FUSE serve loop for vault=%s", vaultName)

	if err := bfs.Serve(c, filesys); err != nil {
		log.Printf("ERROR FUSE Serve: %v", err)
	}

	log.Printf("MountVault finished vault=%s duration=%s", vaultName, time.Since(start))

	return nil
}

func (vd *VaultDaemon) UnmountVault(vaultName string) error {

	log.Printf("UnmountVault requested: %s", vaultName)

	vd.mu.Lock()
	mountPath, ok := vd.mounts[vaultName]
	vd.mu.Unlock()

	if !ok {
		log.Printf("Vault not mounted: %s", vaultName)
		return nil
	}

	log.Printf("Executing: umount -l %s", mountPath)

	cmd := exec.Command("umount", "-l", mountPath)

	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("ERROR umount failed: %v output=%s", err, string(out))
		return err
	}

	log.Printf("Unmount successful: %s", mountPath)

	vd.mu.Lock()
	delete(vd.mounts, vaultName)
	vd.mu.Unlock()

	log.Printf("Vault removed from map: %s", vaultName)

	return nil
}

func main() {
		fmt.Println(`
 __                            __   ______           
|  \                          |  \ /      \          
| $$  ______    _______   ____| $$|  $$$$$$\ _______ 
| $$ /      \  /       \ /      $$| $$_  \$$/       \
| $$|  $$$$$$\|  $$$$$$$|  $$$$$$$| $$ \   |  $$$$$$$
| $$| $$    $$ \$$    \ | $$  | $$| $$$$    \$$    \ 
| $$| $$$$$$$$ _\$$$$$$\| $$__| $$| $$      _\$$$$$$\
| $$ \$$     \|       $$ \$$    $$| $$     |       $$
 \$$  \$$$$$$$ \$$$$$$$   \$$$$$$$ \$$      \$$$$$$$ 
	`)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting Lesdfs Vault Daemon v0.1")

	vd := NewVaultDaemon()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {

		<-ctx.Done()

		log.Println("Shutdown signal received")

		vd.mu.Lock()

		for name := range vd.mounts {

			log.Printf("Auto-unmounting vault: %s", name)

			err := vd.UnmountVault(name)
			if err != nil {
				log.Printf("ERROR auto-unmount: %v", err)
			}

		}

		vd.mu.Unlock()

		log.Println("Daemon shutdown complete")

		os.Exit(0)
	}()

	http.HandleFunc("/mount", func(w http.ResponseWriter, r *http.Request) {

		vault := r.URL.Query().Get("vault")
		vaultPath := r.URL.Query().Get("path")
		password := r.URL.Query().Get("password")

		log.Printf("HTTP /mount request vault=%s path=%s", vault, vaultPath)

		if vault == "" || vaultPath == "" || password == "" {

			log.Println("HTTP /mount missing parameters")

			w.WriteHeader(400)
			w.Write([]byte("missing parameters"))
			return
		}

		go func() {

			err := vd.MountVault(vault, vaultPath, password)

			if err != nil {
				log.Printf("MountVault error: %v", err)
			}

		}()

		w.Write([]byte("ok"))
	})

	http.HandleFunc("/unmount", func(w http.ResponseWriter, r *http.Request) {

		vault := r.URL.Query().Get("vault")

		log.Printf("HTTP /unmount request vault=%s", vault)

		if vault == "" {

			log.Println("HTTP /unmount missing vault name")

			w.WriteHeader(400)
			w.Write([]byte("missing vault name"))
			return
		}

		err := vd.UnmountVault(vault)

		if err != nil {

			log.Printf("HTTP unmount failed: %v", err)

			http.Error(w, "failed to unmount: "+err.Error(), 500)
			return
		}

		w.Write([]byte("ok"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:8080")

	if err != nil {
		log.Fatal("Cannot listen:", err)
	}

	log.Println("Daemon listening on 127.0.0.1:8080")

	err = http.Serve(listener, nil)

	if err != nil {
		log.Fatal(err)
	}
}