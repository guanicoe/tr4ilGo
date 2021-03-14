package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
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

var (
	msg        string
	LogLvl     string
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

	DBName    = flag.String("d", "creds.db", "Name of the database.")
	Path      = flag.String("u", "/media/parrot/HASHDB", "Path where the raw leak files are.")
	NWorkers  = flag.Int("w", 50, "Number of workers to go scan files. Each worker will scrap one text file at a time.")
	Parent    = flag.String("p", "Collection 1", "Name of the parent directory")
	CleanDB   = flag.Bool("r", false, "Delets the database to start fresh. NO RETURN")
	LogLevel  = flag.String("v", "", "Log level [default: WARN | v: INFO | vv: DEBUG ]")
	BatchSize = flag.Int("b", 1000, "Batch size when inserting to database. When scrapping the file list, a slice is made and when it reaches a given size, a batch INSERT is made to the database.")
)

func main() {

	if !tui.Effects() {
		fmt.Printf("\n\nWARNING: This terminal does not support colours, view will be very limited.\n\n")
	}

	ASCIIArt()

	flag.Parse()

	switch {
	case *LogLevel == "":
		log.SetLevel(log.WarnLevel)
		LogLvl = "Warning"
	case *LogLevel == "v":
		log.SetLevel(log.InfoLevel)
		LogLvl = "Info"
	case *LogLevel == "vv":
		log.SetLevel(log.DebugLevel)
		log.SetReportCaller(true)
		LogLvl = "Debug"
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	printParam()
	if *CleanDB {
		os.Remove(*DBName)
		Logg(fmt.Sprintf("Database '%s' was successfully deleted", *DBName), "Warn")
	}

	if _, err := os.Stat(*DBName); os.IsNotExist(err) {
		Logg(fmt.Sprintf("Database does not exist - creating %s...", *DBName), "Warn")

		file, err := os.Create(*DBName) // Create SQLite file
		CheckErr(err, "Fatal", "Could not create database file")
		file.Close()

		err = CreateTable()
		CheckErr(err, "Fatal", "Could not create tables")

	}

	db, _ := sql.Open("sqlite3", fmt.Sprintf("./%s", *DBName))
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

	wd := filepath.Join(*Path, *Parent)
	dirs, err := ioutil.ReadDir(wd)

	CheckErr(err, "Fatal", fmt.Sprint("Could not open directory:", wd))

	for _, d := range dirs {
		if !(strings.Contains(d.Name(), ".tar")) {

			dirS = dirStruct{parent: *Parent,
				name: d.Name(),
				path: filepath.Join(wd, d.Name()),
			}

			files, err := ioutil.ReadDir(dirS.path)
			if err != nil {
				CheckErr(err, "Error", fmt.Sprint("Could not open directory:", dirS.path))
				continue
			}

			for _, f := range files {
				if strings.Contains(f.Name(), ".txt") {
					dirS.file = f.Name()

					h.Write([]byte(fmt.Sprint(dirS.parent, dirS.name, dirS.file)))
					hash := hex.EncodeToString(h.Sum(nil))

					id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)

					if err != nil {
						Logg(fmt.Sprint("adding file to db: ", dirS.parent, dirS.name, dirS.file, " ", id), "Debug")
						lineNum, err = LineCounter(filepath.Join(*Path, dirS.parent, dirS.name, dirS.file))
						CheckErr(err, "Warn", "Trying to count number of lines in file")

						err = InsertRow(param.DB, leaksTable, []leakRows{{Name: dirS.name,
							Parent:     dirS.parent,
							FileName:   dirS.file,
							HashID:     hash,
							Date:       fmt.Sprint(time.Now()),
							Website:    "reddit",
							LineNumber: lineNum,
							Status:     1}})

						CheckErr(err, "Warn", fmt.Sprintf("Could not add row"))
						id, err = GetForeignKey(param.DB, "leaks", "hashID", hash)
						CheckErr(err, "Warn", fmt.Sprintf("Could not get leakid"))

					}

					dirS.leakID = id
				}

				status, err = ReadStatus(param.DB, id)
				CheckErr(err, "Warn", fmt.Sprintf("Could not get leaks status with id %v, ", id))
				if status != 3 {
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
