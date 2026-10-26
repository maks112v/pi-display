package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Public Google Drive folders are read without an API key or sign-in: the
// embeddable folder view lists every entry, and the direct download endpoint
// reports each file's size and mtime, which is all mirror needs.

func isDrive(source string) bool { return strings.HasPrefix(source, "https://drive.google.com/") }

var driveIDPattern = regexp.MustCompile(`^[\w-]{10,}$`)

// parseDriveFolder pulls the folder ID out of a share link such as
// https://drive.google.com/drive/folders/<id>?usp=sharing.
func parseDriveFolder(source string) (string, error) {
	u, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	_, id, _ := strings.Cut(u.Path, "/folders/")
	id, _, _ = strings.Cut(id, "/")
	if u.Host != "drive.google.com" || !driveIDPattern.MatchString(id) {
		return "", fmt.Errorf("%q is not a Google Drive folder link (https://drive.google.com/drive/folders/...)", source)
	}
	return id, nil
}

// driveClient bounds how long Google can stall before answering, without
// capping how long a large video takes to download.
var driveClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: t}
}()

func connectDrive(ctx context.Context, source string) (fs.FS, func(), error) {
	id, err := parseDriveFolder(source)
	if err != nil {
		return nil, nil, err
	}
	return &driveFS{ctx: ctx, folder: id, ids: map[string]string{}}, func() {}, nil
}

// driveFS is the top level of a public Drive folder as an fs.FS. Only files
// are listed; subfolders and native Google Docs are skipped. Open only finds
// files seen by the last ReadDir, which is how mirror uses it.
type driveFS struct {
	ctx    context.Context
	folder string
	ids    map[string]string // name -> file ID
}

func (d *driveFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	resp, err := d.get(http.MethodGet, "https://drive.google.com/embeddedfolderview?id="+d.folder)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	page, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	files, err := parseDriveListing(string(page))
	if err != nil {
		return nil, err
	}
	entries := make([]fs.DirEntry, len(files))
	for i, f := range files {
		d.ids[f.name] = f.id
		entries[i] = driveEntry{fs: d, file: f}
	}
	return entries, nil
}

func (d *driveFS) Open(name string) (fs.File, error) {
	id, ok := d.ids[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	resp, err := d.download(http.MethodGet, id)
	if err != nil {
		return nil, err
	}
	return driveFile{ReadCloser: resp.Body, info: driveInfo(name, resp)}, nil
}

func (d *driveFS) download(method, id string) (*http.Response, error) {
	resp, err := d.get(method, "https://drive.usercontent.google.com/download?export=download&confirm=t&id="+url.QueryEscape(id))
	if err != nil {
		return nil, err
	}
	// Quota and permission problems come back as HTML pages, not file bytes.
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		_ = resp.Body.Close()
		return nil, errors.New("drive returned a web page instead of the file (is it shared publicly, or over its download quota?)")
	}
	return resp, nil
}

func (d *driveFS) get(method, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(d.ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := driveClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New("drive folder or file not found (is it shared as \"Anyone with the link\"?)")
		}
		return nil, fmt.Errorf("drive: %s", resp.Status)
	}
	return resp, nil
}

type driveListing struct{ id, name string }

var driveEntryPattern = regexp.MustCompile(`(?s)class="flip-entry" id="entry-([\w-]+)".*?href="([^"]*)".*?class="flip-entry-title">([^<]*)<`)

// parseDriveListing extracts the files from an embeddedfolderview page. Drive
// allows duplicate names in one folder; the first one listed wins.
func parseDriveListing(page string) ([]driveListing, error) {
	// A private folder redirects to a sign-in page, which has no listing.
	if !strings.Contains(page, `id="flip-contents"`) {
		return nil, errors.New("drive folder is not public (share it as \"Anyone with the link\")")
	}
	var files []driveListing
	seen := map[string]bool{}
	for _, m := range driveEntryPattern.FindAllStringSubmatch(page, -1) {
		name := html.UnescapeString(m[3])
		if !strings.Contains(m[2], "drive.google.com/file/d/") || seen[name] {
			continue
		}
		seen[name] = true
		files = append(files, driveListing{id: m[1], name: name})
	}
	return files, nil
}

// driveEntry fetches size and mtime only when asked, so mirror's filter keeps
// non-media files from costing a request.
type driveEntry struct {
	fs   *driveFS
	file driveListing
}

func (e driveEntry) Name() string      { return e.file.name }
func (e driveEntry) IsDir() bool       { return false }
func (e driveEntry) Type() fs.FileMode { return 0 }

func (e driveEntry) Info() (fs.FileInfo, error) {
	resp, err := e.fs.download(http.MethodHead, e.file.id)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", e.file.name, err)
	}
	_ = resp.Body.Close()
	return driveInfo(e.file.name, resp), nil
}

func driveInfo(name string, resp *http.Response) fileInfo {
	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	modTime, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return fileInfo{name: name, size: size, modTime: modTime}
}

type driveFile struct {
	io.ReadCloser
	info fileInfo
}

func (f driveFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) Mode() fs.FileMode  { return 0o444 }
func (i fileInfo) ModTime() time.Time { return i.modTime }
func (i fileInfo) IsDir() bool        { return false }
func (i fileInfo) Sys() any           { return nil }
