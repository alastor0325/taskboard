package reviewserver

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
)

var portByPlatform = map[string]int{
	"darwin":  7777,
	"windows": 7778,
}

func Port() int {
	if p, ok := portByPlatform[runtime.GOOS]; ok {
		return p
	}
	return 7779
}

// Start launches the review server on the platform port.
func Start() error {
	port := Port()
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRoot)
	addr := "localhost:" + strconv.Itoa(port)
	return http.ListenAndServe(addr, mux)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fragment := r.URL.Fragment
	if fragment == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, indexPage())
		return
	}
	// Serve patch diff for the worktree identified by fragment.
	diff, err := patchDiff(fragment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, diff)
}

func patchDiff(worktreeFragment string) (string, error) {
	worktree := "~/firefox-" + worktreeFragment
	out, err := exec.Command("git", "-C", worktree,
		"log", "--oneline", "origin/main..HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	diff, err := exec.Command("git", "-C", worktree,
		"diff", "origin/main..HEAD").Output()
	if err != nil {
		return string(out), nil
	}
	return string(out) + "\n" + string(diff), nil
}

func indexPage() string {
	return `<!DOCTYPE html><html><body>
<p>taskboard review server — append <code>#worktree-fragment</code> to the URL to view a diff.</p>
</body></html>`
}
