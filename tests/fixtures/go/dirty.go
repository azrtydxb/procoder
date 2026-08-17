// Deliberately unsafe/broken fixture — exercises every go.js finding id.
//
// Shares a directory (and so a package) with clean.go — see the note there.
package fixtures

import (
	"crypto/md5"
	"crypto/tls"
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
)

func doWork() (int, error) {
	return 1, nil
}

func ignoresError() {
	result, _ := doWork()
	fmt.Println(result)
}

func lookupUser(db *sql.DB, id string) (*sql.Rows, error) {
	return db.Query(fmt.Sprintf("SELECT * FROM t WHERE id = %s", id))
}

func runCommand(userInput string) {
	exec.Command("sh", "-c", userInput).Run()
}

func insecureConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

func hashPassword(password string) [16]byte {
	return md5.Sum([]byte(password))
}

func makeToken() int64 {
	token := rand.Int63()
	return token
}

func mustNotHappen() {
	panic("unreachable")
}

func fetchPage(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func debugPrint(x int) {
	fmt.Println("here", x)
}

func sendRequest(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func lookupUserTainted(db *sql.DB, id string) (*sql.Rows, error) {
	q := "SELECT * FROM t WHERE id = " + id
	return db.Query(q)
}
