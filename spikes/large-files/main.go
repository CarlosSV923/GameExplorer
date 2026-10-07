//go:build linux

// Command spike is a THROWAWAY prototype for phase 0.5.
//
// It validates, on the real NAS, that the planned stack handles game files of
// tens of gigabytes: resumable tus uploads, 7zz extraction, rename into the
// library, ranged downloads and streamed "store" zips, while keeping memory flat.
// None of this code is meant to be reused as-is in apps/api.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

//go:embed index.html
var static embed.FS

type upload struct {
	ID        string    `json:"id"`
	FileName  string    `json:"fileName"`
	Size      int64     `json:"size"`
	Path      string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	Seconds   float64   `json:"seconds,omitempty"`
	MBps      float64   `json:"mbps,omitempty"`
	Done      bool      `json:"done"`
}

type job struct {
	ID        string    `json:"id"`
	UploadID  string    `json:"uploadId"`
	Status    string    `json:"status"` // running | done | failed | moved
	Percent   int       `json:"percent"`
	OutDir    string    `json:"outDir"`
	Started   time.Time `json:"started"`
	Seconds   float64   `json:"seconds,omitempty"`
	MBps      float64   `json:"mbps,omitempty"`
	Error     string    `json:"error,omitempty"`
	MovedTo   string    `json:"movedTo,omitempty"`
	MoveMs    int64     `json:"moveMs,omitempty"`
	MoveEXDEV bool      `json:"moveExdev,omitempty"`
	Sample    string    `json:"sample,omitempty"`
	Warning   string    `json:"warning,omitempty"`
	Verified  string    `json:"verified,omitempty"`
	VerifySecs float64  `json:"verifySeconds,omitempty"`
}

type state struct {
	mu      sync.Mutex
	uploads map[string]*upload
	jobs    map[string]*job
	peakRSS int64
}

var (
	library    string
	stagingDir string
	outputDir  string
	st         = &state{uploads: map[string]*upload{}, jobs: map[string]*job{}}
	percentRe  = regexp.MustCompile(`(\d{1,3})%`)
	started    = time.Now()

	startupError string
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	library = envOr("LIBRARY_PATH", "/library")
	stagingDir = filepath.Join(library, ".gameexplorer-spike", "staging")
	outputDir = filepath.Join(library, "spike-output")

	if mask, err := strconv.ParseUint(envOr("UMASK", "002"), 8, 32); err == nil {
		syscall.Umask(int(mask))
	}
	// Startup problems are reported on the web page instead of exiting: on
	// TrueNAS a crash loop makes the container logs very hard to read.
	for _, d := range []string{filepath.Join(stagingDir, "uploads"), outputDir} {
		if err := os.MkdirAll(d, 0o775); err != nil {
			startupError = fmt.Sprintf("No se pudo crear %s como uid=%d gid=%d: %v. "+
				"Revisa que el volumen apunte al dataset correcto y que ese usuario tenga permiso de escritura.",
				d, os.Getuid(), os.Getgid(), err)
			log.Error("startup", "error", startupError)
			break
		}
	}

	tusHandler, err := newTusHandler(log)
	if err != nil && startupError == "" {
		startupError = "tus: " + err.Error()
		log.Error("startup", "error", startupError)
	}

	recoverUploads(log)
	go samplePeakRSS()

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", http.FileServerFS(static))
	if tusHandler != nil {
		mux.Handle("/files/", http.StripPrefix("/files/", tusHandler))
	}
	mux.HandleFunc("GET /api/stats", handleStats)
	mux.HandleFunc("GET /api/state", handleState)
	mux.HandleFunc("POST /api/extract", handleExtract)
	mux.HandleFunc("POST /api/move", handleMove)
	mux.HandleFunc("GET /api/tree", handleTree)
	mux.HandleFunc("GET /download/{path...}", handleDownload)
	mux.HandleFunc("GET /zip/{path...}", handleZip)
	mux.HandleFunc("POST /api/reset", handleReset)

	srv := &http.Server{
		Addr:              ":" + envOr("PORT", "8090"),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Info("spike listening", "addr", srv.Addr, "library", library, "uid", os.Getuid(), "gid", os.Getgid())
	if err := srv.ListenAndServe(); err != nil {
		log.Error("server", "error", err)
		os.Exit(1)
	}
}

func newTusHandler(log *slog.Logger) (*tusd.Handler, error) {
	dir := filepath.Join(stagingDir, "uploads")
	composer := tusd.NewStoreComposer()
	filestore.New(dir).UseIn(composer)
	filelocker.New(dir).UseIn(composer)

	h, err := tusd.NewHandler(tusd.Config{
		BasePath:                "/files/",
		StoreComposer:           composer,
		NotifyCreatedUploads:    true,
		NotifyCompleteUploads:   true,
		RespectForwardedHeaders: true,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		for ev := range h.CreatedUploads {
			st.mu.Lock()
			st.uploads[ev.Upload.ID] = &upload{
				ID:        ev.Upload.ID,
				FileName:  ev.Upload.MetaData["filename"],
				Size:      ev.Upload.Size,
				Path:      ev.Upload.Storage["Path"],
				CreatedAt: time.Now(),
			}
			st.mu.Unlock()
		}
	}()
	go func() {
		for ev := range h.CompleteUploads {
			st.mu.Lock()
			u, ok := st.uploads[ev.Upload.ID]
			if ok {
				u.Done = true
				u.Path = ev.Upload.Storage["Path"]
				u.Seconds = time.Since(u.CreatedAt).Seconds()
				u.MBps = mbps(u.Size, u.Seconds)
				log.Info("upload complete", "id", u.ID, "file", u.FileName, "size", u.Size, "seconds", u.Seconds, "MBps", u.MBps)
			}
			st.mu.Unlock()
		}
	}()
	return h, nil
}

// recoverUploads re-registers uploads already on disk (tusd writes <id>.info
// next to each upload), so restarting the container does not lose them.
func recoverUploads(log *slog.Logger) {
	dir := filepath.Join(stagingDir, "uploads")
	infos, _ := filepath.Glob(filepath.Join(dir, "*.info"))
	for _, p := range infos {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var fi tusd.FileInfo
		if err := json.Unmarshal(b, &fi); err != nil {
			continue
		}
		info, err := os.Stat(strings.TrimSuffix(p, ".info"))
		if err != nil {
			continue
		}
		st.uploads[fi.ID] = &upload{
			ID:        fi.ID,
			FileName:  fi.MetaData["filename"],
			Size:      fi.Size,
			Path:      strings.TrimSuffix(p, ".info"),
			CreatedAt: info.ModTime(),
			Done:      info.Size() == fi.Size,
		}
	}
	log.Info("recovered uploads", "count", len(st.uploads))
}

// ---------- stats ----------

func rssBytes() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				kb, _ := strconv.ParseInt(f[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func samplePeakRSS() {
	for range time.Tick(500 * time.Millisecond) {
		r := rssBytes()
		st.mu.Lock()
		if r > st.peakRSS {
			st.peakRSS = r
		}
		st.mu.Unlock()
	}
}

func handleStats(w http.ResponseWriter, _ *http.Request) {
	var fsStat syscall.Statfs_t
	_ = syscall.Statfs(library, &fsStat)
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	st.mu.Lock()
	peak := st.peakRSS
	st.mu.Unlock()
	writeJSON(w, map[string]any{
		"rssBytes":     rssBytes(),
		"peakRssBytes": peak,
		"heapBytes":    ms.HeapAlloc,
		"goroutines":   runtime.NumGoroutine(),
		"diskFree":     int64(fsStat.Bavail) * fsStat.Bsize,
		"diskTotal":    int64(fsStat.Blocks) * fsStat.Bsize,
		"uid":          os.Getuid(),
		"gid":          os.Getgid(),
		"uptimeSec":    int(time.Since(started).Seconds()),
		"has7zz":       has7zz(),
		"library":      library,
		"startupError": startupError,
	})
}

func has7zz() bool {
	_, err := exec.LookPath("7zz")
	return err == nil
}

func handleState(w http.ResponseWriter, _ *http.Request) {
	st.mu.Lock()
	defer st.mu.Unlock()
	ups := make([]*upload, 0, len(st.uploads))
	for _, u := range st.uploads {
		ups = append(ups, u)
	}
	jobs := make([]*job, 0, len(st.jobs))
	for _, j := range st.jobs {
		jobs = append(jobs, j)
	}
	writeJSON(w, map[string]any{"uploads": ups, "jobs": jobs})
}

// ---------- extraction ----------

func handleExtract(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	password := r.FormValue("password")

	st.mu.Lock()
	u, ok := st.uploads[id]
	st.mu.Unlock()
	if !ok || !u.Done {
		http.Error(w, "upload not found or not complete", http.StatusNotFound)
		return
	}

	j := &job{
		ID:       "job-" + id,
		UploadID: id,
		Status:   "running",
		OutDir:   filepath.Join(stagingDir, "extracted", id),
		Started:  time.Now(),
	}
	st.mu.Lock()
	st.jobs[j.ID] = j
	st.mu.Unlock()

	go runExtraction(j, u, password)
	writeJSON(w, j)
}

func runExtraction(j *job, u *upload, password string) {
	args := []string{"x", "-y", "-bsp1", "-bso0", "-bse2", "-o" + j.OutDir}
	if password != "" {
		args = append(args, "-p"+password)
	} else {
		// A non-empty dummy makes 7zz fail fast on encrypted archives instead of prompting.
		args = append(args, "-p-no-password-given-")
	}
	args = append(args, u.Path)

	cmd := exec.Command("7zz", args...)
	cmd.Stdin = nil
	var stderr bytes.Buffer
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		finishJob(j, u, err)
		return
	}
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		finishJob(j, u, err)
		return
	}

	sc := bufio.NewScanner(stdout)
	sc.Split(splitProgress)
	for sc.Scan() {
		if m := percentRe.FindStringSubmatch(sc.Text()); m != nil {
			p, _ := strconv.Atoi(m[1])
			st.mu.Lock()
			j.Percent = p
			st.mu.Unlock()
		}
	}

	err = cmd.Wait()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		switch {
		case onlyAttributeErrors(msg):
			// SMB datasets with NFSv4 ACLs (aclmode=restricted) forbid chmod, so
			// 7zz cannot apply Unix modes stored in the archive. The data itself
			// is written; verifyExtraction below proves it with sizes and CRC32.
			st.mu.Lock()
			j.Warning = "atributos del archivo no aplicados (el dataset no permite chmod)"
			st.mu.Unlock()
			err = nil
		case strings.Contains(msg, "Wrong password") || strings.Contains(msg, "encrypted"):
			err = fmt.Errorf("7zz: %w: encrypted archive: wrong or missing password (%s)", err, msg)
		default:
			err = fmt.Errorf("7zz: %w: %s", err, msg)
		}
	}
	if err == nil {
		err = verifyExtraction(j, u.Path, password)
	}
	finishJob(j, u, err)
}

func onlyAttributeErrors(stderr string) bool {
	found := false
	for line := range strings.SplitSeq(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "Cannot set file attribute") {
			return false
		}
		found = true
	}
	return found
}

// verifyExtraction compares every extracted file with the archive index
// (size and CRC32), streaming each file once.
func verifyExtraction(j *job, archive, password string) error {
	start := time.Now()
	args := []string{"l", "-slt", "-ba"}
	if password != "" {
		args = append(args, "-p"+password)
	} else {
		args = append(args, "-p-no-password-given-")
	}
	out, err := exec.Command("7zz", append(args, archive)...).Output()
	if err != nil {
		return fmt.Errorf("verify: list archive: %w", err)
	}

	type item struct {
		path, crc string
		size      int64
		dir       bool
	}
	var items []item
	var cur item
	flush := func() {
		if cur.path != "" && !cur.dir {
			items = append(items, cur)
		}
		cur = item{}
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), " = ")
		switch {
		case !ok:
			if strings.TrimSpace(line) == "" {
				flush()
			}
		case k == "Path":
			flush()
			cur.path = v
		case k == "Size":
			cur.size, _ = strconv.ParseInt(v, 10, 64)
		case k == "CRC":
			cur.crc = strings.ToUpper(v)
		case k == "Folder":
			cur.dir = v == "+"
		}
	}
	flush()

	okCount := 0
	for _, it := range items {
		p := filepath.Join(j.OutDir, filepath.FromSlash(it.path))
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("verify: %s missing: %w", it.path, err)
		}
		if info.Size() != it.size {
			return fmt.Errorf("verify: %s size %d, want %d", it.path, info.Size(), it.size)
		}
		if it.crc != "" {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			h := crc32.NewIEEE()
			_, err = io.Copy(h, f)
			f.Close()
			if err != nil {
				return err
			}
			if got := fmt.Sprintf("%08X", h.Sum32()); got != it.crc {
				return fmt.Errorf("verify: %s crc %s, want %s", it.path, got, it.crc)
			}
		}
		okCount++
	}

	st.mu.Lock()
	j.Verified = fmt.Sprintf("%d/%d archivos OK (tamaño + CRC32)", okCount, len(items))
	j.VerifySecs = time.Since(start).Seconds()
	st.mu.Unlock()
	return nil
}

// splitProgress splits 7zz progress output, which redraws using \b and \r.
func splitProgress(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == '\n' || b == '\r' || b == '\b' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func finishJob(j *job, u *upload, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	j.Seconds = time.Since(j.Started).Seconds()
	if err != nil {
		j.Status = "failed"
		j.Error = err.Error()
		return
	}
	j.Status = "done"
	j.Percent = 100
	j.Seconds -= j.VerifySecs // report extraction speed and verification time separately
	j.MBps = mbps(dirSize(j.OutDir), j.Seconds)
	// The real app deletes the archive after a successful extraction.
	_ = os.Remove(u.Path)
	_ = os.Remove(u.Path + ".info")
	_ = os.Remove(u.Path + ".lock")
}

// ---------- move into library ----------

func handleMove(w http.ResponseWriter, r *http.Request) {
	st.mu.Lock()
	j, ok := st.jobs[r.URL.Query().Get("job")]
	st.mu.Unlock()
	if !ok || j.Status != "done" {
		http.Error(w, "job not found or not done", http.StatusNotFound)
		return
	}

	target := filepath.Join(outputDir, j.UploadID)
	start := time.Now()
	err := os.Rename(j.OutDir, target)
	exdev := errors.Is(err, syscall.EXDEV)

	st.mu.Lock()
	defer st.mu.Unlock()
	j.MoveMs = time.Since(start).Milliseconds()
	j.MoveEXDEV = exdev
	if err != nil {
		j.Error = "rename: " + err.Error()
		writeJSON(w, j)
		return
	}
	j.Status = "moved"
	j.MovedTo = target
	j.Sample = describeSample(target)
	writeJSON(w, j)
}

// describeSample reports owner and mode of the first file, to verify SMB permissions.
func describeSample(dir string) string {
	var out string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || out != "" {
			return nil
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil
		}
		if sys, ok := info.Sys().(*syscall.Stat_t); ok {
			rel, _ := filepath.Rel(dir, p)
			out = fmt.Sprintf("%s uid=%d gid=%d mode=%s", rel, sys.Uid, sys.Gid, info.Mode())
		}
		return filepath.SkipAll
	})
	return out
}

// ---------- browse & download ----------

type entry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}

func handleTree(w http.ResponseWriter, _ *http.Request) {
	var entries []entry
	_ = filepath.WalkDir(outputDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == outputDir {
			return nil
		}
		rel, _ := filepath.Rel(outputDir, p)
		if strings.Count(rel, string(filepath.Separator)) > 2 {
			return nil
		}
		e := entry{Path: filepath.ToSlash(rel), IsDir: d.IsDir()}
		if info, err := d.Info(); err == nil && !d.IsDir() {
			e.Size = info.Size()
		}
		entries = append(entries, e)
		return nil
	})
	writeJSON(w, entries)
}

// safePath resolves a client path strictly inside outputDir.
func safePath(raw string) (string, bool) {
	p := filepath.Join(outputDir, filepath.FromSlash(raw))
	rel, err := filepath.Rel(outputDir, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return p, true
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	p, ok := safePath(r.PathValue("path"))
	if !ok {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func handleZip(w http.ResponseWriter, r *http.Request) {
	root, ok := safePath(r.PathValue("path"))
	if !ok {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(root)+".zip"))

	zw := zip.NewWriter(w)
	defer zw.Close()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		hdr.Method = zip.Store // no compression: game files barely compress and it would cost CPU
		dst, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(dst, f)
		return err
	})
}

func handleReset(w http.ResponseWriter, _ *http.Request) {
	st.mu.Lock()
	st.uploads = map[string]*upload{}
	st.jobs = map[string]*job{}
	st.peakRSS = 0
	st.mu.Unlock()
	_ = os.RemoveAll(filepath.Join(library, ".gameexplorer-spike"))
	_ = os.RemoveAll(outputDir)
	_ = os.MkdirAll(filepath.Join(stagingDir, "uploads"), 0o775)
	_ = os.MkdirAll(outputDir, 0o775)
	w.WriteHeader(http.StatusNoContent)
}

// ---------- helpers ----------

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func mbps(size int64, seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	return float64(size) / 1e6 / seconds
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
