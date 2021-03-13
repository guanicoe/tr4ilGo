package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

type worker struct {
	CTX       context.Context
	DB        *sql.DB
	Mutex     *sync.Mutex
	ID        int
	Work      chan workRequest
	PugsQueue chan chan workRequest
	Result    chan workOutput
}

// NewWorker creates, and returns a new Worker object. Its only argument
// is a channel that the worker can add itself to whenever it is done its
// work.
func workerNew(ctx context.Context, id int, db *sql.DB, mutex *sync.Mutex, pugsQueue chan chan workRequest, result chan workOutput) worker {

	worker := worker{
		CTX:       ctx,
		DB:        db,
		Mutex:     mutex,
		ID:        id,
		Work:      make(chan workRequest),
		PugsQueue: pugsQueue,
		Result:    result,
	}

	return worker
}

func (w *worker) Start() {

	go func() {
		for {
			w.PugsQueue <- w.Work
			select {
			case <-w.CTX.Done():
				return
			case work := <-w.Work:
				err := processFile(work, w)
				r := workOutput{
					Work:  work,
					Error: err,
				}
				w.Result <- r
			}
		}
	}()
}

func processFile(work workRequest, w *worker) error {

	filePath := filepath.Join(work.Job.path, work.Job.file)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)

	data := []credRows{}
	var email, password, username, domain, seperator string
	h := sha1.New()
	// re := regexp.MustCompile(`.+@+\w+\.{1}\w+`)
	err = ChangeStatus(w.DB, 2, work.Job.leakID)
	CheckErr(err, "Error", "Trying to change leaks status so 1")
	for scanner.Scan() {
		line := scanner.Text()
		// email = re.MatchString(line)
		// password = strings.Split(line, email)[1]
		switch {
		case strings.Contains(line, ":"):
			seperator = ":"
		case strings.Contains(line, ";"):
			seperator = ";"
		}
		split := strings.Split(line, seperator)
		if len(split) == 2 {
			email = split[0]
			password = split[1]
			split = strings.Split(email, "@")
			if len(split) == 2 {
				username = split[0]
				domain = split[1]
			} else {
				continue
			}
		} else {
			continue
		}

		h.Write([]byte(fmt.Sprint(email, password)))
		hash := hex.EncodeToString(h.Sum(nil))

		w.Mutex.Lock()
		id := 0
		id, err = GetForeignKey(w.DB, "creds", "hashID", hash)
		CheckErr(err, "Debug", "Could not get foreignkey for creds hasgID")
		w.Mutex.Unlock()
		if id == 0 {
			w.Mutex.Lock()
			id, err = GetForeignKey(w.DB, "hosts", "domain", domain)
			if err != nil {
				// log.Println(fmt.Sprintf("Could not get row : %s", err))
				err = InsertRow(w.DB, hostsTable, []hostRows{{Domain: domain}})
				CheckErr(err, "Warn", fmt.Sprintf("Could not add row : %s ||| line: %s", err, line))
				id, err = GetForeignKey(w.DB, "hosts", "domain", domain)
				CheckErr(err, "Warn", fmt.Sprintf("Could not GetForeignKey : %s ||| line: %s", err, line))
			}

			w.Mutex.Unlock()
			leakID := 0
			data = append(data, credRows{Email: email, HashID: hash, Username: username, Password: password, FirstSeen: fmt.Sprint(time.Now()), Host: id, Leak: leakID})
		}
		if len(data) > 1000 {
			w.Mutex.Lock()
			err = InsertRow(w.DB, credsTable, data)
			w.Mutex.Unlock()
			CheckErr(err, "Warn", fmt.Sprintf("Could not add row : %s, ", err))
			data = []credRows{}
		}

	}

	return nil
}
