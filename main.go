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

type dbTable struct {
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
	hostsTable = dbTable{
		columns:   "domain, smtp, smtpPort, imap, imapPort",
		questions: "?, ?, ?, ?, ?",
		name:      "hosts",
	}

	leaksTable = dbTable{
		columns:   "name, parent, filename, hashID, date, website, linenumber, status",
		questions: "?, ?, ?, ?, ?, ?, ?, ?",
		name:      "leaks",
	}

	credsTable = dbTable{
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
		if err != nil {
			log.Fatal(err.Error())
		}
		file.Close()

		CreateTable()

		msg = tui.Blue("creds.db created")
		log.Debug(msg)
	}

	sqliteDatabase, _ := sql.Open("sqlite3", fmt.Sprintf("./%s", DBName))

	param := JobParam{
		DB:    sqliteDatabase,
		Mutex: &sync.Mutex{}}
	scanWorkingDir(param)
	defer sqliteDatabase.Close()
}

func scanWorkingDir(param JobParam) {
	h := sha1.New()
	var id int
	var lineNum int
	var dirS dirStruct

	msg = tui.Blue("Indexing raw files...")
	log.Debug(msg)

	sliceDir := []dirStruct{}

	wd := filepath.Join(cwd, parent)
	dirs, err := ioutil.ReadDir(wd)

	if err != nil {
		msg = tui.Red(fmt.Sprint("Could not open directory:", wd, err))
		log.Fatal(msg)
	}

	for _, d := range dirs {
		if !(strings.Contains(d.Name(), "tar")) {

			dirS = dirStruct{parent: parent,
				name: d.Name(),
				path: filepath.Join(wd, d.Name()),
			}

			files, err := ioutil.ReadDir(dirS.path)
			if err != nil {
				msg = tui.Red(fmt.Sprint("Could not open directory: ", dirS.path, err))
				log.Error(msg)
				continue
			}

			for _, f := range files {
				if strings.Contains(f.Name(), "txt") {
					dirS.file = f.Name()
					// add to leaks data base and grab leakID
					h.Write([]byte(fmt.Sprint(dirS.parent, dirS.name, dirS.file)))
					hash := hex.EncodeToString(h.Sum(nil))
					id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)

					if err != nil {
						log.Debug("adding file to db: ", dirS.file, " ", id)
						path := filepath.Join(cwd, dirS.parent, dirS.name, dirS.file)
						lineNum, err = LineCounter(path)
						CheckErr(err, "Warn", "Trying to count number of lines in file")
						// log.Println(fmt.Sprintf("Could not get row : %s", err))
						err = InsertRow(param.DB, leaksTable, []leakRows{{Name: dirS.name,
							Parent:     dirS.parent,
							FileName:   dirS.file,
							HashID:     hash,
							Date:       fmt.Sprint(time.Now()),
							Website:    "reddit",
							LineNumber: lineNum,
							Status:     0}})
						if err != nil {
							log.Warn(fmt.Sprintf("Could not add row : %s", err))
						}
						id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)
						if err != nil {
							log.Warn(fmt.Sprintf("Could not add row : %s", err))
						}
					}

					dirS.leakID = id
				}
				var status int
				status, err = ReadStatus(param.DB, id)
				if status != 2 {
					sliceDir = append(sliceDir, dirS)
				}

			}

		}

	}

	param.JobList = sliceDir

	msg = tui.Green("Starting job!")
	log.Info(msg)

	startTime := time.Now()
	startProducer(&param)
	endTime := time.Now()
	timeDelta := endTime.Sub(startTime)

	msg = tui.Green(fmt.Sprintf("Finished job at %s - It took %s", endTime, timeDelta))
	log.Info(msg)

}
