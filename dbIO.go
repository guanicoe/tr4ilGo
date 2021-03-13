package main

import (
	"database/sql"
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
)

func CreateTable() {
	var statement *sql.Stmt
	var err error
	db, err := sql.Open("sqlite3", fmt.Sprintf("./%s", DBName)) // Open the created SQLite File
	defer db.Close()                                            // Defer Closing the database

	createhostsTableSQL := `CREATE TABLE hosts (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"domain" TEXT UNIQUE,
		"smtp" TEXT,
		"smtpPort" INTEGER,
		"imap" TEXT,
		"imapPort" INTEGER 		
	  );` // SQL Statement for Create Table

	log.Debug("Create hosts table...")
	statement, err = db.Prepare(createhostsTableSQL) // Prepare SQL Statement
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec() // Execute SQL Statements
	log.Debug("hosts table created")

	createLeaksTableSQL := `CREATE TABLE leaks (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"name" TEXT NOT NULL,
		"parent" TEXT NOT NULL,
		"filename" TEXT NOT NULL, 
		"hashID" TEXT UNIQUE NOT NULL,
		"date" TEXT,
		"website" TEXT,
		"linenumber" INTEGER,
		"status" INTEGER
	  );` // SQL Statement for Create Table

	log.Debug("Create leaks table...")
	statement, err = db.Prepare(createLeaksTableSQL) // Prepare SQL Statement
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec() // Execute SQL Statements
	log.Debug("hosts table created")

	createCredTableSQL := `CREATE TABLE creds (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"email" TEXT NOT NULL,
		"username" TEXT,
		"password" TEXT,
		"hashID" TEXT NOT NULL UNIQUE,
		"valid" INTEGER NOT NULL DEFAULT 0,
		"host" INTEGER, 
		"firstSeen" TEXT,
		"leak" INTEGER, 
		FOREIGN KEY(host) REFERENCES hosts(id),
		FOREIGN KEY(leak) REFERENCES leaks(id)
	  );` // SQL Statement for Create Table

	log.Debug("Create creds table...")
	statement, err = db.Prepare(createCredTableSQL) // Prepare SQL Statement
	if err != nil {
		log.Fatal(err.Error())
	}
	statement.Exec() // Execute SQL Statements
	log.Debug("creds table created")
}

// We are passing db reference connection from main to our method with other parameters
func InsertRow(db *sql.DB, tab dbTable, row interface{}) (err error) {
	numRows := reflect.ValueOf(row).Len()
	// log.Println(fmt.Sprintf("Inserting %s record ...", tab.name))
	insertSQL := fmt.Sprintf("INSERT INTO %s(%s) VALUES", tab.name, tab.columns)
	valuesSQL := fmt.Sprintf(" (%s)", tab.questions)
	for j := 0; j < numRows-1; j++ {
		valuesSQL = fmt.Sprintf("%s, (%s)", valuesSQL, tab.questions)
	}

	insertSQL = fmt.Sprint(insertSQL, valuesSQL)
	statement, err := db.Prepare(insertSQL) // Prepare statement. This is good to avoid SQL injections
	if err != nil {
		return err
	}

	var args []interface{}
	for j := 0; j < numRows; j++ {
		rv := reflect.ValueOf(row).Index(j)
		for i := 0; i < rv.NumField(); i++ {
			args = append(args, rv.Field(i).Interface())
		}
	}
	_, err = statement.Exec(args...)

	if err != nil {
		return err
	}

	return nil
}

func GetForeignKey(db *sql.DB, tab, col, val string) (id int, err error) {
	// var id int
	row, err := db.Query(fmt.Sprintf("SELECT id FROM %s WHERE %s = '%s';", tab, col, val))
	if err != nil {
		return -1, err
	}
	defer row.Close()

	for row.Next() { // Iterate and fetch the records from result cursor
		row.Scan(&id)
	}
	if id == 0 {
		return 0, fmt.Errorf("No rows where found for %s", val)
	}
	return id, nil

}

func ChangeStatus(db *sql.DB, val, leakid int) (err error) {
	// var id int
	// update userinfo set username=? where uid=?
	insertSQL := fmt.Sprintf("update leaks set status=? where id=?")
	statement, err := db.Prepare(insertSQL) // Prepare statement. This is good to avoid SQL injections

	if err != nil {
		return err
	}
	_, err = statement.Exec(val, leakid)
	if err != nil {
		return err
	}

	return nil

}

func ReadStatus(db *sql.DB, id int) (status int, err error) {
	// var id int
	row, err := db.Query(fmt.Sprintf("SELECT status FROM leaks WHERE id=%s;", id))
	if err != nil {
		return -1, err
	}
	defer row.Close()

	for row.Next() { // Iterate and fetch the records from result cursor
		row.Scan(&status)
	}
	if status == 0 {
		return -1, fmt.Errorf("No rows where found for %s", id)
	}
	return status, nil

}
