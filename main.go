package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/evilsocket/islazy/tui"
	log "github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

type JobParam struct {
	DB      *sql.DB
	Mutex   *sync.Mutex
	JobList []dirStruct
}

type DBTable struct {
	columns   string // = "domain, smtp, smtpPort, imap, imapPort"
	questions string //  = "?, ?, ?, ?, ?"
	name      string // host
}

type hostRows struct {
	Domain   string
	Smtp     string
	SmtpPort int
	Imap     string
	ImapPort int
}

type leakRows struct {
	Name       string
	Parent     string
	FileName   string
	HashID     string
	Date       string
	Website    string
	LineNumber int
	Status     int // 0: never read; 1: started; 2: finished
}

type credRows struct {
	Email     string
	Username  string
	Password  string
	HashID    string
	Valid     int
	Host      int
	FirstSeen string
	Leak      int
}

const (
	DBName     = "creds.db"
	cwd        = "/media/parrot/HASH DB"
	parent     = "Collection 1"
	numWorkers = 50
)

var (
	msg        string
	hostsTable = DBTable{
		columns:   "domain, smtp, smtpPort, imap, imapPort",
		questions: "?, ?, ?, ?, ?",
		name:      "hosts",
	}

	leaksTable = DBTable{
		columns:   "name, parent, filename, hashID, date, website, linenumber, status",
		questions: "?, ?, ?, ?, ?, ?, ?, ?",
		name:      "leaks",
	}

	credsTable = DBTable{
		columns:   "email, username, password, hashID, valid, host, firstSeen, leak",
		questions: "?, ?, ?, ?, ?, ?, ?, ?",
		name:      "creds",
	}
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	os.Remove(DBName)

	if _, err := os.Stat(DBName); os.IsNotExist(err) {
		msg = tui.Yellow("Database does not exist - creating creds.db...")
		log.Warn(msg)

		file, err := os.Create(DBName) // Create SQLite file
		CheckErr(err, "Fatal", "Could not create database file")
		file.Close()

		err = CreateTable()
		CheckErr(err, "Fatal", "Could not create tables")

	}

	db, _ := sql.Open("sqlite3", fmt.Sprintf("./%s", DBName))
	defer db.Close()

	param := JobParam{
		DB:    db,
		Mutex: &sync.Mutex{}}

	scanWorkingDir(param)

}

func scanWorkingDir(param JobParam) {
	h := sha1.New()
	var id int
	var lineNum int
	var status int
	var dirS dirStruct

	Logg("Indexing raw files...", "Debug")

	sliceDir := []dirStruct{}

	wd := filepath.Join(cwd, parent)
	dirs, err := ioutil.ReadDir(wd)

	CheckErr(err, "Fatal", fmt.Sprint("Could not open directory:", wd))

	for _, d := range dirs {
		if !(strings.Contains(d.Name(), "tar")) {

			dirS = dirStruct{parent: parent,
				name: d.Name(),
				path: filepath.Join(wd, d.Name()),
			}

			files, err := ioutil.ReadDir(dirS.path)
			if err != nil {
				CheckErr(err, "Error", fmt.Sprint("Could not open directory:", dirS.path))
				continue
			}

			for _, f := range files {
				if strings.Contains(f.Name(), "txt") {
					dirS.file = f.Name()

					h.Write([]byte(fmt.Sprint(dirS.parent, dirS.name, dirS.file)))
					hash := hex.EncodeToString(h.Sum(nil))

					id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)

					if err != nil {
						Logg(fmt.Sprint("adding file to db: ", dirS.file, " ", id), "Debug")
						lineNum, err = LineCounter(filepath.Join(cwd, dirS.parent, dirS.name, dirS.file))
						CheckErr(err, "Warn", "Trying to count number of lines in file")

						err = InsertRow(param.DB, leaksTable, []leakRows{{Name: dirS.name,
							Parent:     dirS.parent,
							FileName:   dirS.file,
							HashID:     hash,
							Date:       fmt.Sprint(time.Now()),
							Website:    "reddit",
							LineNumber: lineNum,
							Status:     0}})

						CheckErr(err, "Warn", fmt.Sprintf("Could not add row"))

						id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)

						CheckErr(err, "Warn", fmt.Sprintf("Could not get leakid"))

					}

					dirS.leakID = id
				}

				status, err = ReadStatus(param.DB, id)
				CheckErr(err, "Warn", fmt.Sprintf("Could not get leaks status"))
				if status != 2 {
					sliceDir = append(sliceDir, dirS)
				}

			}

		}

	}

	param.JobList = sliceDir

	Logg("Stating job!", "Info")

	startTime := time.Now()
	startProducer(&param)
	endTime := time.Now()
	timeDelta := endTime.Sub(startTime)

	Logg(fmt.Sprintf("Finished job at %s - It took %s", endTime, timeDelta), "Info")

}
