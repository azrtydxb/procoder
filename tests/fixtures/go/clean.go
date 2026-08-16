// Clean fixture — real near-misses for every rule go.js implements, all of
// which must stay silent.
package clean

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

func doWork() (int, error) {
	return 1, nil
}

func handlesError() error {
	result, err := doWork()
	if err != nil {
		return err
	}
	log.Println("worked", result)
	return nil
}

// The blank identifier here discards the loop value, not an error — the
// single highest false-positive risk for true/ignored-error.
func sumIndexes(xs []int) int {
	total := 0
	for i, _ := range xs {
		total += i
	}
	return total
}

func countKeys(m map[string]int) int {
	count := 0
	for k, _ := range m {
		count += len(k)
		_ = count
	}
	return count
}

func lookupUser(db *sql.DB, id string) (*sql.Rows, error) {
	return db.Query("SELECT * FROM t WHERE id = $1", id)
}

func runCommand(dir string) error {
	return exec.Command("ls", dir).Run()
}

func hashPassword(password string) [32]byte {
	return sha256.Sum256([]byte(password))
}

func makeToken() ([]byte, error) {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	return buf, err
}

func mustNotPanic() error {
	return fmt.Errorf("something went wrong")
}

func fetchPage(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return resp, nil
}

func startup() {
	log.Println("started")
}
