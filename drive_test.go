package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestParseDriveFolder(t *testing.T) {
	for _, link := range []string{
		"https://drive.google.com/drive/folders/15uNXeRBIhVvZJIhL4yTw4IsStMhUaaxl",
		"https://drive.google.com/drive/folders/15uNXeRBIhVvZJIhL4yTw4IsStMhUaaxl?usp=sharing",
		"https://drive.google.com/drive/u/1/folders/15uNXeRBIhVvZJIhL4yTw4IsStMhUaaxl/",
	} {
		if id, err := parseDriveFolder(link); err != nil || id != "15uNXeRBIhVvZJIhL4yTw4IsStMhUaaxl" {
			t.Errorf("parseDriveFolder(%q) = %q, %v", link, id, err)
		}
	}
	for _, bad := range []string{
		"https://drive.google.com/file/d/13AhW1Z1GYGaiTpJ0Pr2TTXoQivb6jx-a/view",
		"https://drive.google.com/drive/folders/",
		"https://evil.example/drive/folders/15uNXeRBIhVvZJIhL4yTw4IsStMhUaaxl",
	} {
		if _, err := parseDriveFolder(bad); err == nil {
			t.Errorf("parseDriveFolder(%q): expected error", bad)
		}
	}
}

// Trimmed from a real embeddedfolderview response.
const driveListingPage = `<div id="flip-contents" class="flip-list-view">` +
	`<div class="flip-entry" id="entry-1oow464Z4Et95RAwtRZn6dLOFb_YYpLwL" tabindex="0" role="link"><div class="flip-entry-info"><a href="https://drive.google.com/drive/folders/1oow464Z4Et95RAwtRZn6dLOFb_YYpLwL" target="_blank"><div class="flip-entry-title">sub</div></a></div></div>` +
	`<div class="flip-entry" id="entry-1TFYNzuZJTgNGzGmjraZ58ZVOh9_YoKeBnU-opWgXQL4" tabindex="0" role="link"><div class="flip-entry-info"><a href="https://docs.google.com/document/d/1TFYNzuZJTgNGzGmjraZ58ZVOh9_YoKeBnU-opWgXQL4/edit?usp=drive_web" target="_blank"><div class="flip-entry-title">notes.docx</div></a></div></div>` +
	`<div class="flip-entry" id="entry-13AhW1Z1GYGaiTpJ0Pr2TTXoQivb6jx-a" tabindex="0" role="link"><div class="flip-entry-info"><a href="https://drive.google.com/file/d/13AhW1Z1GYGaiTpJ0Pr2TTXoQivb6jx-a/view?usp=drive_web" target="_blank"><div class="flip-entry-visual"></div><div class="flip-entry-title">Tom &amp; Jerry.jpg</div></a></div></div>` +
	`<div class="flip-entry" id="entry-1cKSdgtWrPgvEsBGmjWALOH33taGyVXKb" tabindex="0" role="link"><div class="flip-entry-info"><a href="https://drive.google.com/file/d/1cKSdgtWrPgvEsBGmjWALOH33taGyVXKb/view?usp=drive_web" target="_blank"><div class="flip-entry-title">Tom &amp; Jerry.jpg</div></a></div></div>` +
	`</div>`

func TestParseDriveListing(t *testing.T) {
	got, err := parseDriveListing(driveListingPage)
	if err != nil {
		t.Fatal(err)
	}
	want := []driveListing{
		{id: "1oow464Z4Et95RAwtRZn6dLOFb_YYpLwL", name: "sub", dir: true},
		{id: "13AhW1Z1GYGaiTpJ0Pr2TTXoQivb6jx-a", name: "Tom & Jerry.jpg"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := parseDriveListing(`<html><title>Sign in</title></html>`); err == nil {
		t.Error("sign-in page: expected error")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDriveFSSkipsUnsafeNames(t *testing.T) {
	entry := func(id, name string) string {
		return `<div class="flip-entry" id="entry-` + id + `"><a href="https://drive.google.com/file/d/` + id + `/view"><div class="flip-entry-title">` + name + `</div></a></div>`
	}
	page := `<div id="flip-contents">` + entry("good", "01.jpg") + entry("evil", "x/../01.jpg") + `</div>`
	saved := driveClient
	defer func() { driveClient = saved }()
	var downloads []string
	driveClient = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "download") {
			downloads = append(downloads, r.URL.Query().Get("id"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(page))}, nil
	})}

	d := &driveFS{ctx: context.Background(), entries: map[string]driveListing{".": {id: "root", dir: true}, "sub": {id: "sub", dir: true}}}
	entries, err := d.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "01.jpg" {
		t.Errorf("entries = %v, want only 01.jpg", entries)
	}
	f, err := d.Open("01.jpg")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := d.Open("sub"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open(sub) = %v, want not exist", err)
	}
	if !slices.Equal(downloads, []string{"good"}) {
		t.Errorf("downloads = %v, want only good", downloads)
	}
}
