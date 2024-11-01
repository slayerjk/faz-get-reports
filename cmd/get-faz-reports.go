package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/slayerjk/faz-get-reports/internal/dboperations"
	fazrep "github.com/slayerjk/faz-get-reports/internal/fazrequests"
	ldap "github.com/slayerjk/faz-get-reports/internal/ldap"
	logging "github.com/slayerjk/faz-get-reports/internal/logging"
	"github.com/slayerjk/faz-get-reports/internal/vafswork"
	// "github.com/slayerjk/faz-get-reports/internal/mailing"
	// "github.com/slayerjk/faz-get-reports/internal/hd-naumen-api"
)

const (
	appName               = "faz-get-report"
	dbTable               = "Data"
	dbValueColumn         = "Value"
	dbProcessedColumn     = "Processed"
	dbProcessedDateColumn = "Processed_Date"
)

type fazData struct {
	LdapBindUser    string              `json:"ldap-bind-user"`
	LdapBindPass    string              `json:"ldap-bind-pass"`
	LdapFqdn        string              `json:"ldap-fqdn"`
	LdapBaseDn      string              `json:"ldap-basedn"`
	FazUrl          string              `json:"faz-url"`
	ApiUser         string              `json:"api-user"`
	ApiUserPass     string              `json:"api-user-pass"`
	FazAdom         string              `json:"faz-adom"`
	FazDevice       string              `json:"faz-device"`
	FazDatasetAll   string              `json:"faz-dataset-connections"`
	FazDatasetTotal string              `json:"faz-dataset-total"`
	FazReportName   string              `json:"faz-report-name"`
	FazDatasets     []map[string]string `json:"faz-datasets"`
}

type User struct {
	Username     string
	StartDate    string
	EndDate      string
	UserInitials string
}

func main() {
	// TODO: maybe refactor to be in fazrequests?
	var (
		logsPath      = vafswork.GetExePath() + "/logs" + "_get-faz-reports"
		dataFilePath  = vafswork.GetExePath() + "/data/data.json"
		usersFilePath = vafswork.GetExePath() + "/data/users.csv"
		resultsPath   = vafswork.GetExePath() + "/Reports"
		dbFile        = vafswork.GetExePath() + "/data/data.db"
		// mailingFile   = vafswork.GetExePath() + "/data/mailing.json"

		fazData         fazData
		user            User
		users           []User
		sessionid       string
		reportFilePath  string
		repStartTime    string
		repEndTime      string
		sAMAccountName  string
		fazReportLayout int
		tempList        []string
	)

	// flags
	logDir := flag.String("log-dir", logsPath, "set custom log dir")
	logsToKeep := flag.Int("keep-logs", 7, "set number of logs to keep after rotation")
	mode := flag.String("mode", "db", "set program mode('csv' - use data/users.csv; 'db' - use sqlite3 data/data.db)")
	flag.Parse()

	// logging
	logFile, err := logging.StartLogging(appName, *logDir, *logsToKeep)
	if err != nil {
		log.Fatalf("failed to start logging:\n\t%s", err)
	}

	defer logFile.Close()

	// starting programm notification
	startTime := time.Now()
	log.Println("Program Started")

	// READING FAZ DATA FILE
	fazDataFile, errFile := os.Open(dataFilePath)
	if errFile != nil {
		log.Fatal("FAILED: to open data-file:\n\t", errFile)
	}
	defer fazDataFile.Close()

	byteFazData, errRead := io.ReadAll(fazDataFile)
	if errRead != nil {
		log.Fatal("FAILED: to io.ReadALL", errRead)
	}

	errJson := json.Unmarshal(byteFazData, &fazData)
	if errJson != nil {
		log.Fatal("FAILED: to unmarshall json:\n\t", errJson)
	}

	// CREATING REPORTS DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		log.Fatal("FAILED: to Create Reports Dir", err)
	}

	// different workflows for mode 'db'(default) & 'csv'
	switch *mode {
	case "db":
		// getting list of unporcessed values in db
		unprocessedValues, err := dboperations.GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn)
		if err != nil {
			log.Fatalf("failed to get list of unprocessed values in db(%s):\n\t%v", dbFile, err)
		}
		fmt.Println(unprocessedValues)

		// exit program if there are no values to process
		if len(unprocessedValues) == 0 {
			log.Fatalf("no values to process this time, exiting")
		}
	case "csv":
		// READING USERS FILE
		usersFile, errFile := os.Open(usersFilePath)

		if errFile != nil {
			log.Fatal("FAILED: to open users file:\n\t", errFile)
		}
		defer usersFile.Close()

		csvreader := csv.NewReader(usersFile)

		for {
			row, err := csvreader.Read()
			if err == io.EOF {
				break
			}

			user.Username = row[0]

			// GETTING USER INITIALS
			tempList = []string{}
			for ind, item := range strings.Split(row[0], " ") {
				if ind == 0 {
					tempList = append(tempList, item)
				} else {
					runeItem := []rune(item)
					tempList = append(tempList, fmt.Sprintf("%s.", string(runeItem[0:1])))
				}
			}
			user.UserInitials = strings.Join(tempList, " ")

			user.StartDate = row[1]
			user.EndDate = row[2]

			users = append(users, user)
		}
	}

	os.Exit(0)

	// TEST update value
	// errU := dboperations.UpdDbValue(dbFile, dbTable, dbValueColumn, dbProcessedColumn, dbProcessedDateColumn, "test7", 0)
	// if errU != nil {
	// 	log.Fatalf("failed to update value(%s) to result(%v):\n\t%v", "test7", 1, errU)
	// }
	// log.Printf("succeeded to update value(%s) to result(%v):\n", "test7", 1)

	// GETTING FAZ REPORT LAYOUT
	sessionid, errS := fazrep.GetSessionid(fazData.FazUrl, fazData.ApiUser, fazData.ApiUserPass)
	if errS != nil {
		log.Fatal("FAILED: to get sessionid\n\t", errS)
	}

	fazReportLayout, errLayout := fazrep.GetFazReportLayout(fazData.FazUrl, sessionid, fazData.FazAdom, fazData.FazReportName)
	if err != nil {
		log.Fatalf("FAILED: to get report layout:\n\t%v", errLayout)
	}

	// STARTING GETTING REPORT LOOP
	for _, user := range users {
		log.Printf("STARTED: GETTING REPORT JOB: %s\n", user.Username)

		// GETTING ADUSER FULL NAME
		adSearchResult, err := ldap.LdapBindAndSearch(user.Username, fazData.LdapFqdn, fazData.LdapBaseDn, fazData.LdapBindUser, fazData.LdapBindPass)
		if err != nil {
			log.Fatal("FAILED: to Fetch AD Full Username:\n\t", err)
		}
		// adSearchResult.Entries[0].Print()
		sAMAccountName = adSearchResult.Entries[0].GetAttributeValue("sAMAccountName")
		// fmt.Println(sAMAccountName)
		log.Printf("User's sAMAccountName found: %s", sAMAccountName)

		// GETTING SESSIONID
		sessionid, err := fazrep.GetSessionid(fazData.FazUrl, fazData.ApiUser, fazData.ApiUserPass)
		if err != nil {
			log.Fatal("FAILED: to get sessionid\n\t", err)
		}

		// UPDATING DATASETS QUERIE
		errUpdDataset := fazrep.UpdateDatasets(fazData.FazUrl, sessionid, fazData.FazAdom, sAMAccountName, fazData.FazDatasets)
		if errUpdDataset != nil {
			log.Fatal("FAILED: to update datasets:\n\t", errUpdDataset)
		}

		// STARTING REPORT
		log.Printf("STARTED: running report job: %s\n", user.Username)

		repId, err := fazrep.StartReport(fazData.FazUrl, fazData.FazAdom, fazData.FazDevice, sessionid, user.StartDate, user.EndDate, fazReportLayout)
		if err != nil {
			log.Fatal("FAILED: to start report", err)
		}

		// DOWNLOADING PDF REPORT
		log.Printf("STARTED: downloading for %s\n", user.Username)

		repData, err := fazrep.DownloadPdfReport(fazData.FazUrl, fazData.FazAdom, sessionid, repId)
		if err != nil {
			log.Fatal("FAILED: to dowonload report:\n\t", err)
		}

		// GETTING DATES FOR REPORT FILE
		tempTime, err := time.Parse("15:04:05 2006/01/02", user.StartDate)
		if err != nil {
			log.Fatal("FAILED: to Parse User Start Time:\n\t", err)
		}
		repStartTime = tempTime.Format("02-01-2006-T-15-04-05")

		tempTime, err = time.Parse("15:04:05 2006/01/02", user.EndDate)
		if err != nil {
			log.Fatal("FAILED: to Parse User End Time:\n\t", err)
		}
		repEndTime = tempTime.Format("02-01-2006-T-15-04-05")

		// SAVING REPORT TO FILE
		dec, err := base64.StdEncoding.DecodeString(repData)
		if err != nil {
			log.Fatal("FAILED: to Decode Report Data:\n\t", err)
		}

		reportFilePath = fmt.Sprintf("%s/%s_%s_%s.zip", resultsPath, user.UserInitials, repStartTime, repEndTime)
		file, err := os.Create(reportFilePath)
		if err != nil {
			log.Fatal("FAILED: to Create Report Blank File:\n\t", err)
		}
		defer file.Close()

		if _, err := file.Write(dec); err != nil {
			log.Fatal("FAILED: to Write Report Data to File:\n\t", err)
		}
		if err := file.Sync(); err != nil {
			log.Fatal("FAILED: to Sync Written Report File:\n\t", err)
		}

		log.Printf("FINISHED: GETTING REPORT JOB: %s\n\n", user.Username)
	}

	// count & print estimated time
	endTime := time.Now()
	log.Printf("Program Done\n\tEstimated time is %f seconds", endTime.Sub(startTime).Seconds())

	// close logfile and rotate logs
	logFile.Close()

	if err := vafswork.RotateFilesByMtime(*logDir, *logsToKeep); err != nil {
		log.Fatalf("failed to rotate logs:\n\t%s", err)
	}
}
