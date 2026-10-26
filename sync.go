package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

const syncInterval = 30 * time.Second

func isSMB(source string) bool { return strings.HasPrefix(source, "smb://") }

// isRemote reports whether source is mirrored into the cache rather than
// played in place.
func isRemote(source string) bool { return isSMB(source) || isDrive(source) }

// connect opens a remote source as an fs.FS for mirror.
func connect(ctx context.Context, d Device) (fs.FS, func(), error) {
	if isDrive(d.Source) {
		return connectDrive(ctx, d.Source)
	}
	return connectSMB(ctx, d)
}

type smbTarget struct {
	addr  string // host:port to dial
	share string // UNC share path, \\host\share
	dir   string // folder inside the share, "" for its root
}

func parseSMB(source string) (smbTarget, error) {
	u, err := url.Parse(source)
	if err != nil {
		return smbTarget{}, err
	}
	if u.Scheme != "smb" || u.Hostname() == "" {
		return smbTarget{}, fmt.Errorf("%q is not smb://host/share[/path]", source)
	}
	share, dir, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	if share == "" {
		return smbTarget{}, fmt.Errorf("%q is missing a share name", source)
	}
	port := u.Port()
	if port == "" {
		port = "445"
	}
	return smbTarget{
		addr:  net.JoinHostPort(u.Hostname(), port),
		share: `\\` + u.Hostname() + `\` + share,
		dir:   dir,
	}, nil
}

// connectSMB logs in to the share and returns its folder as an fs.FS. Each
// sync uses a fresh connection, so network changes never leave a stale one.
func connectSMB(ctx context.Context, d Device) (fs.FS, func(), error) {
	target, err := parseSMB(d.Source)
	if err != nil {
		return nil, nil, err
	}
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", target.addr)
	if err != nil {
		return nil, nil, err
	}
	user := d.Username
	if user == "" {
		user = "guest"
	}
	dialer := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: user, Password: d.Password}}
	session, err := dialer.DialContext(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	share, err := session.WithContext(ctx).Mount(target.share)
	if err != nil {
		_ = session.Logoff()
		_ = conn.Close()
		return nil, nil, err
	}
	closeFn := func() {
		_ = share.Umount()
		_ = session.Logoff()
		_ = conn.Close()
	}
	return shareFS{share: share.WithContext(ctx), dir: target.dir}, closeFn, nil
}

// shareFS adapts an SMB share folder to fs.FS so mirror can be tested
// against an in-memory filesystem.
type shareFS struct {
	share *smb2.Share
	dir   string
}

func (s shareFS) Open(name string) (fs.File, error) {
	return s.share.Open(path.Join(s.dir, name))
}

func (s shareFS) ReadDir(name string) ([]fs.DirEntry, error) {
	infos, err := s.share.ReadDir(path.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	entries := make([]fs.DirEntry, len(infos))
	for i, info := range infos {
		entries[i] = fs.FileInfoToDirEntry(info)
	}
	return entries, nil
}

func syncLoop(ctx context.Context, d Device, dir string) {
	var lastErr string
	for {
		err := syncOnce(ctx, d, dir)
		if msg := fmt.Sprint(err); err != nil && msg != lastErr {
			log.Printf("sync: %v", err)
			lastErr = msg
		} else if err == nil && lastErr != "" {
			log.Print("sync: recovered")
			lastErr = ""
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(syncInterval):
		}
	}
}

func syncOnce(ctx context.Context, d Device, dir string) error {
	src, closeFn, err := connect(ctx, d)
	if err != nil {
		return err
	}
	defer closeFn()
	return mirror(src, dir)
}

// mirror makes dst match the media files and display.toml at the top level
// of src (an SMB share or a public Drive folder). Files are compared by size and mtime, downloaded to a hidden temp
// file, then renamed into place so the player never sees a partial file.
func mirror(src fs.FS, dst string) error {
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return err
	}

	var errs []error
	keep := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !wanted(name) {
			continue
		}
		keep[name] = true
		info, err := e.Info()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		local, err := os.Stat(filepath.Join(dst, name))
		if err == nil && local.Size() == info.Size() && local.ModTime().Unix() == info.ModTime().Unix() {
			continue
		}
		if err := download(src, name, dst, info.ModTime()); err != nil {
			errs = append(errs, fmt.Errorf("download %s: %w", name, err))
		}
	}

	// dst belongs to us, so anything not in the source (including leftover
	// temp files from an interrupted download) gets removed.
	locals, err := os.ReadDir(dst)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, e := range locals {
		if !keep[e.Name()] {
			if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// wanted rejects anything that could escape dst or isn't something we play.
func wanted(name string) bool {
	return fs.ValidPath(name) && !strings.ContainsAny(name, `/\`) && !strings.HasPrefix(name, ".") &&
		(isMedia(name) || name == showFile)
}

func download(src fs.FS, name, dst string, modTime time.Time) error {
	in, err := src.Open(name)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := filepath.Join(dst, ".partial-"+name)
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chtimes(tmp, modTime, modTime)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dst, name))
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}
