package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnassignedSection(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "psp", "Daxter", "d.iso", "daxter", map[string]any{})

	// A game moved to the section is an entry named after its folder, which
	// remembers its console.
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(game)+"/unassign", ""), http.StatusNoContent, "unassign")
	e := entryNamed(t, srv, cookie, "Daxter")
	if !e.Folder || e.Console == nil || *e.Console != "psp" || len(e.Files) != 1 ||
		e.Files[0].Path != "Daxter/Daxter.iso" || e.Files[0].Reason != "manual" || len(e.Files[0].Consoles) != 2 || e.Files[0].Archive {
		t.Fatalf("entry = %+v", e)
	}
	if exists(filepath.Join(library, "psp")) {
		t.Fatal("empty folders left behind")
	}
	res := get(t, srv, cookie, "/api/unassigned/"+id(e.Files[0].ID)+"/download")
	if res.StatusCode != http.StatusOK || body(t, res) != "daxter" {
		t.Fatalf("download = %d", res.StatusCode)
	}
	if z := readZip(t, get(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/download")); z["Daxter/Daxter.iso"] != "daxter" {
		t.Fatalf("entry zip = %+v", z)
	}

	// Assigning starts a job; the files stay in the section, busy.
	res = post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/assign", `{"console":"wii","title":"Jak"}`)
	wantStatus(t, res, http.StatusCreated, "assign")
	job := decode[jobJSON](t, res)
	if !job.FromUnassigned {
		t.Fatalf("job = %+v", job)
	}
	waitStatus(t, srv, cookie, job.ID, "confirm")
	if files := jobFiles(t, srv, cookie, job.ID); len(files) != 1 || !files[0].InPlace || !files[0].Valid {
		t.Fatalf("job files = %+v", files)
	}
	if !entryNamed(t, srv, cookie, "Daxter").Busy {
		t.Fatal("an entry being assigned must be busy")
	}
	wantStatus(t, post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/trash", ""), http.StatusConflict, "trash a busy entry")
	wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/unassigned/"+id(e.Files[0].ID), ""), http.StatusConflict, "delete a busy file")
	wantStatus(t, post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/assign", `{"console":"wii","title":"Jak"}`), http.StatusConflict, "assign twice")

	// Cancelling frees it, where it was.
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/cancel", ""), http.StatusOK, "cancel")
	if e := entryNamed(t, srv, cookie, "Daxter"); e.Busy || e.Files[0].Path != "Daxter/Daxter.iso" {
		t.Fatalf("entry after cancel = %+v", e)
	}

	// Assign again and store it: it leaves the section.
	res = post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/assign", `{"console":"wii","title":"Jak"}`)
	job = decode[jobJSON](t, res)
	waitStatus(t, srv, cookie, job.ID, "confirm")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/commit", commitBody(t, map[string]any{"path": "Daxter/Daxter.iso"})), http.StatusOK, "commit")
	if got := readFile(t, filepath.Join(library, "wii", "Jak", "Jak.iso")); got != "daxter" {
		t.Fatalf("stored = %q", got)
	}
	if len(unassignedEntries(t, srv, cookie)) != 0 || exists(filepath.Join(library, "_unassigned", "Daxter")) {
		t.Fatal("stored entry must leave the section")
	}
	if !exists(filepath.Join(library, "_unassigned")) {
		t.Fatal("the unassigned folder always exists")
	}

	// Trash and delete, by file and by entry.
	writeSettled(t, filepath.Join(library, "_unassigned", "a.z64"), "n64")
	writeSettled(t, filepath.Join(library, "_unassigned", "b.rar"), "rar")
	writeSettled(t, filepath.Join(library, "_unassigned", "Pack", "c.iso"), "iso")
	writeSettled(t, filepath.Join(library, "_unassigned", "Pack", "d.txt"), "txt")
	entries := unassignedEntries(t, srv, cookie)
	if len(entries) != 3 {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		switch e.Name {
		case "a.z64":
			if e.Folder || len(e.Files[0].Consoles) != 0 || e.Files[0].Archive {
				t.Errorf("a.z64 = %+v", e)
			}
			wantStatus(t, post(t, srv, cookie, "/api/unassigned/"+id(e.Files[0].ID)+"/trash", ""), http.StatusNoContent, "trash file")
		case "b.rar":
			if !e.Files[0].Archive {
				t.Errorf("b.rar = %+v", e)
			}
			wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/unassigned/entries/"+id(e.ID), ""), http.StatusNoContent, "delete entry")
		case "Pack":
			if !e.Folder || len(e.Files) != 2 || e.Size != 6 {
				t.Errorf("Pack = %+v", e)
			}
			wantStatus(t, post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/trash", ""), http.StatusNoContent, "trash entry")
		}
	}
	if len(unassignedEntries(t, srv, cookie)) != 0 || exists(filepath.Join(library, "_unassigned", "b.rar")) || exists(filepath.Join(library, "_unassigned", "Pack")) {
		t.Fatal("section not empty")
	}
	trash := trashList(t, srv, cookie)
	if len(trash) != 2 {
		t.Fatalf("trash = %+v", trash)
	}
	for _, e := range trash {
		if len(e.Files) == 2 {
			return // the folder is one trash entry
		}
	}
	t.Fatalf("trash = %+v", trash)
}

func TestFilesCopiedIntoTheUnassignedFolder(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)

	// Just copied: shown at once, but still copying and without id.
	path := filepath.Join(library, "_unassigned", "Copy", "game.nsp")
	writeFile(t, path, "part")
	e := entryNamed(t, srv, cookie, "Copy")
	if !e.Copying || e.ID != 0 || len(e.Files) != 1 || !e.Files[0].Copying {
		t.Fatalf("entry while copying = %+v", e)
	}
	// Old, but changed since the previous read: still copying.
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if e = entryNamed(t, srv, cookie, "Copy"); !e.Copying {
		t.Fatalf("changed entry = %+v", e)
	}
	// Unchanged since then: it joins the section.
	e = entryNamed(t, srv, cookie, "Copy")
	if e.Copying || e.ID == 0 || e.Files[0].Reason != "samba" || len(e.Files[0].Consoles) != 1 {
		t.Fatalf("settled entry = %+v", e)
	}
}

func TestUploadWithoutConsole(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)

	// Extracted whole, junk included, under the title (RF-07b).
	archive := zipOf(t, map[string]string{"Splatoon 3 [v0].xci": "base", "info.nfo": "nfo"})
	first := upload(t, srv, cookie, "splatoon.zip", "", "Splatoon 3", archive, "igdbId", "1234")
	waitStatus(t, srv, cookie, first, "unassigned")
	if got := readFile(t, filepath.Join(library, "_unassigned", "Splatoon 3", "Splatoon 3 [v0].xci")); got != "base" {
		t.Fatalf("base = %q", got)
	}
	if !exists(filepath.Join(library, "_unassigned", "Splatoon 3", "info.nfo")) {
		t.Fatal("junk must be kept: without console nothing tells it apart")
	}
	// Another file of the same game joins the entry, whatever the case.
	second := upload(t, srv, cookie, "Splatoon 3 [v2752512].nsp", "", "splatoon 3", []byte("update"))
	waitStatus(t, srv, cookie, second, "unassigned")
	e := entryNamed(t, srv, cookie, "Splatoon 3")
	if len(e.Files) != 3 || e.IgdbID == nil || *e.IgdbID != 1234 || e.Console != nil {
		t.Fatalf("entry = %+v", e)
	}

	// The whole entry is assigned at once; "No guardar" keeps a file.
	res := post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/assign", `{"console":"switch","title":"Splatoon 3"}`)
	wantStatus(t, res, http.StatusCreated, "assign")
	job := decode[jobJSON](t, res)
	waitStatus(t, srv, cookie, job.ID, "confirm")
	valid := 0
	for _, f := range jobFiles(t, srv, cookie, job.ID) {
		if !f.InPlace {
			t.Errorf("file %+v must stay in place", f)
		}
		if f.Valid {
			valid++
		}
	}
	if valid != 2 {
		t.Fatalf("valid files = %d", valid)
	}
	res = post(t, srv, cookie, "/api/jobs/"+job.ID+"/commit", commitBody(t,
		map[string]any{"path": "Splatoon 3/Splatoon 3 [v0].xci", "kind": "base"},
		map[string]any{"path": "Splatoon 3/Splatoon 3 [v2752512].nsp", "skip": true},
	))
	wantStatus(t, res, http.StatusOK, "commit")
	if got := readFile(t, filepath.Join(library, "switch", "Splatoon 3", "Splatoon 3 [BASE].xci")); got != "base" {
		t.Fatalf("stored = %q", got)
	}
	if e := entryNamed(t, srv, cookie, "Splatoon 3"); e.Busy || len(e.Files) != 2 {
		t.Fatalf("entry after commit = %+v", e)
	}

	// "No guardar" is only for the unassigned section.
	plain := upload(t, srv, cookie, "p.nsp", "switch", "Plain", []byte("p"))
	waitStatus(t, srv, cookie, plain, "confirm")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+plain+"/commit", commitBody(t, map[string]any{"path": "p.nsp", "skip": true})), http.StatusBadRequest, "skip on an upload")
}

func TestAssignAnEntryWithArchives(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)
	dir := filepath.Join(library, "_unassigned", "Zelda")
	writeSettled(t, filepath.Join(dir, "zelda.zip"), string(zipOf(t, map[string]string{"Zelda.iso": "iso", "notes.txt": "notes"})))
	writeSettled(t, filepath.Join(dir, "manual.pdf"), "pdf")
	writeSettled(t, filepath.Join(dir, "Zelda (copy).iso"), "copy")
	e := entryNamed(t, srv, cookie, "Zelda")

	res := post(t, srv, cookie, "/api/unassigned/entries/"+id(e.ID)+"/assign", `{"console":"wii","title":"Zelda"}`)
	wantStatus(t, res, http.StatusCreated, "assign")
	job := decode[jobJSON](t, res)
	// Two game files for a one-file console: the user keeps one.
	waitStatus(t, srv, cookie, job.ID, "confirm")
	inPlace := map[string]bool{}
	for _, f := range jobFiles(t, srv, cookie, job.ID) {
		inPlace[f.Path] = f.InPlace
	}
	if len(inPlace) != 4 || inPlace["Zelda/Zelda.iso"] || !inPlace["Zelda/Zelda (copy).iso"] || !inPlace["Zelda/manual.pdf"] {
		t.Fatalf("job files = %+v", inPlace)
	}
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/commit", commitBody(t,
		map[string]any{"path": "Zelda/Zelda.iso"}, map[string]any{"path": "Zelda/Zelda (copy).iso"},
	)), http.StatusBadRequest, "two files for Wii")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/commit", commitBody(t,
		map[string]any{"path": "Zelda/Zelda.iso"}, map[string]any{"path": "Zelda/Zelda (copy).iso", "skip": true},
	)), http.StatusOK, "commit")

	if got := readFile(t, filepath.Join(library, "wii", "Zelda", "Zelda.iso")); got != "iso" {
		t.Fatalf("stored = %q", got)
	}
	// The archive is gone; what was not stored stays in the entry.
	if exists(filepath.Join(dir, "zelda.zip")) || !exists(filepath.Join(dir, "notes.txt")) || !exists(filepath.Join(dir, "manual.pdf")) {
		t.Fatal("entry after commit: archive must go, the rest must stay")
	}
	if e := entryNamed(t, srv, cookie, "Zelda"); e.Busy || len(e.Files) != 3 {
		t.Fatalf("entry after commit = %+v", e)
	}
}
