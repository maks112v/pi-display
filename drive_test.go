package main

import (
	"slices"
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
	want := []driveListing{{id: "13AhW1Z1GYGaiTpJ0Pr2TTXoQivb6jx-a", name: "Tom & Jerry.jpg"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if _, err := parseDriveListing(`<html><title>Sign in</title></html>`); err == nil {
		t.Error("sign-in page: expected error")
	}
}
