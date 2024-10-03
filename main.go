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

	fazrep "github.com/slayerjk/faz-get-reports/internal/fazrequests"
	ldap "github.com/slayerjk/faz-get-reports/internal/ldap"
	logging "github.com/slayerjk/faz-get-reports/internal/logging"
	rotatefiles "github.com/slayerjk/faz-get-reports/internal/rotatefiles"
)

const (
	defaultLogPath    = "logs"
	defaultLogsToKeep = 7
	appName           = "faz-get-report"
	dataFilePath      = "data/data.json"
	usersFilePath     = "data/users.csv"
	resultsPath       = "Results"
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
	var (
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
	logDir := flag.String("log-dir", defaultLogPath, "set custom log dir")
	logsToKeep := flag.Int("keep-logs", defaultLogsToKeep, "set number of logs to keep after rotation")
	flag.Parse()

	// logging
	appName := "faz-get-requests"

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

	// CREATING RESULT DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		log.Fatal("FAILED: to Create Results Dir", err)
	}

	// GETTING REPORT LAYOUT
	sessionid, errS := fazrep.GetSessionid(fazData.FazUrl, fazData.ApiUser, fazData.ApiUserPass)
	if errS != nil {
		log.Fatal("FAILED: to get sessionid\n\t", errS)
	}

	fazReportLayout, errLayout := fazrep.GetFazReportLayout(fazData.FazUrl, sessionid, fazData.FazAdom, fazData.FazReportName)
	if err != nil {
		log.Fatalf("FAILED: to get report layout:\n\t%v", errLayout)
	}

	// GETTING REPORT LOOP
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

	if err := rotatefiles.RotateFilesByMtime(*logDir, *logsToKeep); err != nil {
		log.Fatalf("failed to rotate logs:\n\t%s", err)
	}
}
