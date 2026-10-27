---
title: Google Drive
description: Play from a Google Drive folder shared by link.
---

Any Google Drive folder can be a source, as long as anyone with the link can view it. There's no sign-in on the Pi.

## Share the folder

1. In Google Drive, right-click the folder and choose **Share**.
2. Under **General access**, pick **Anyone with the link** with the **Viewer** role.
3. Choose **Copy link**.

The link looks like this:

```text
https://drive.google.com/drive/folders/1Fq8xK2vLmNabcdEFGhij?usp=sharing
```

Paste it when setup asks for the media folder.

> Anyone with the link can see the files. Don't put anything private in the folder.

## Syncing

Like [SMB shares](/docs/smb), the folder is mirrored onto the Pi every 30 seconds and played from that copy, so the slideshow keeps going if the internet drops.

## Limits

- Only files at the top of the folder are used. Subfolders are skipped.
- Native Google Docs, Sheets, and Slides are ignored. Export slides as images first.
- If two files share a name, the first one Drive lists wins.
- pi-display reads Drive's public folder page and download links, not an official API. If Google changes those pages, Drive sources can break until pi-display is updated.
