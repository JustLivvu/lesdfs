package vaultfs

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"bazil.org/fuse"
	bfs "bazil.org/fuse/fs"
)

func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

func ReadPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	pass, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return pass[:len(pass)-1], nil
}


func InitSalt(vaultPath string) ([]byte, error) {
	saltFile := filepath.Join(vaultPath, ".salt")
	if _, err := os.Stat(saltFile); os.IsNotExist(err) {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return nil, err
		}
		if err := os.WriteFile(saltFile, salt, 0600); err != nil {
			return nil, err
		}
		return salt, nil
	}
	return os.ReadFile(saltFile)
}

type VaultFS struct {
	VaultPath string
	Key       []byte
}

func (v *VaultFS) Root() (bfs.Node, error) {
	return &Dir{VaultPath: v.VaultPath, Key: v.Key}, nil
}

type Dir struct {
	VaultPath string
	Key       []byte
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0700 | os.ModeDir
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getgid())
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (bfs.Node, error) {
	fullPath := filepath.Join(d.VaultPath, name)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fuse.ENOENT
	}
	if info.IsDir() {
		return &Dir{VaultPath: fullPath, Key: d.Key}, nil
	}
	return &File{Path: fullPath, Key: d.Key}, nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	entries, err := os.ReadDir(d.VaultPath)
	if err != nil {
		return nil, err
	}
	var result []fuse.Dirent
	for _, e := range entries {
		if e.Name() == ".salt" {
			continue
		}
		ent := fuse.Dirent{Name: e.Name()}
		if e.IsDir() {
			ent.Type = fuse.DT_Dir
		} else {
			ent.Type = fuse.DT_File
		}
		result = append(result, ent)
	}
	return result, nil
}

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (bfs.Node, error) {
	fullPath := filepath.Join(d.VaultPath, req.Name)
	if err := os.Mkdir(fullPath, 0700); err != nil {
		return nil, err
	}
	return &Dir{VaultPath: fullPath, Key: d.Key}, nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (bfs.Node, bfs.Handle, error) {
	fullPath := filepath.Join(d.VaultPath, req.Name)
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}
	file.Close()
	node := &File{Path: fullPath, Key: d.Key}
	return node, node, nil
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	fullPath := filepath.Join(d.VaultPath, req.Name)
	info, err := os.Stat(fullPath)
	if err != nil {
		return fuse.ENOENT
	}
	if info.IsDir() {
		return os.RemoveAll(fullPath)
	}
	return os.Remove(fullPath)
}


type File struct {
	Path string
	Key  []byte
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	info, err := os.Stat(f.Path)
	if err != nil {
		return err
	}
	a.Mode = 0600
	a.Size = uint64(info.Size())
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getgid())
	return nil
}

func (f *File) ReadAll(ctx context.Context) ([]byte, error) {
	cipherData, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, err
	}
	if len(cipherData) < 12 {
		return nil, errors.New("file corrupted")
	}
	return decrypt(f.Key, cipherData)
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
    data, err := f.ReadAll(ctx)
    if err != nil {
        data = []byte{}
    }
    newData := make([]byte, max(len(data), int(req.Offset)+len(req.Data)))
    copy(newData, data)
    copy(newData[req.Offset:], req.Data)

    cipherData, err := encrypt(f.Key, newData)
    if err != nil {
        return err
    }
    if err := os.WriteFile(f.Path, cipherData, 0600); err != nil {
        return err
    }
    resp.Size = len(req.Data)
    return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < aesgcm.NonceSize() {
		return nil, errors.New("invalid data")
	}
	nonce := data[:aesgcm.NonceSize()]
	ciphertext := data[aesgcm.NonceSize():]
	return aesgcm.Open(nil, nonce, ciphertext, nil)
}