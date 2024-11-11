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
	"regexp"
	"strings"
	"time"

	"github.com/slayerjk/faz-get-reports/internal/dboperations"
	fazrep "github.com/slayerjk/faz-get-reports/internal/fazrequests"
	ldap "github.com/slayerjk/faz-get-reports/internal/ldap"
	logging "github.com/slayerjk/faz-get-reports/internal/logging"
	"github.com/slayerjk/faz-get-reports/internal/vafswork"
	// "github.com/slayerjk/faz-get-reports/internal/mailing"
)

const (
	appName               = "faz-get-report"
	dbTable               = "Data"
	dbValueColumn         = "Value"
	dbProcessedColumn     = "Processed"
	dbProcessedDateColumn = "Processed_Date"
)

type fazData struct {
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

type ldapData struct {
	LdapBindUser string `json:"ldap-bind-user"`
	LdapBindPass string `json:"ldap-bind-pass"`
	LdapFqdn     string `json:"ldap-fqdn"`
	LdapBaseDn   string `json:"ldap-basedn"`
}

type naumenData struct {
	NaumenBaseUrl   string `json:"naumen-base-url"`
	NaumenAccessKey string `json:"naumen-access-key"`
}

type User struct {
	Username     string
	StartDate    string
	EndDate      string
	UserInitials string
	// Fields below is only for mode 'naumen'
	DBId        string
	ServiceCall string
	RP          string
}

func main() {
	// TODO: maybe refactor to be in fazrequests?
	var (
		logsPath           = vafswork.GetExePath() + "/logs" + "_" + appName
		fazDataFilePath    = vafswork.GetExePath() + "/data/faz-data.json"
		ldapDataFilePath   = vafswork.GetExePath() + "/data/ldap-data.json"
		naumenDataFilePath = vafswork.GetExePath() + "/data/naumen-data.json"
		usersFilePath      = vafswork.GetExePath() + "/data/users.csv"
		resultsPath        = vafswork.GetExePath() + "/Reports"
		dbFile             = vafswork.GetExePath() + "/data/data.db"
		// mailingFile   = vafswork.GetExePath() + "/data/mailing.json"

		fazData         fazData
		ldapData        ldapData
		naumenData      naumenData
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
	logsDir := flag.String("log-dir", logsPath, "set custom log dir")
	logsToKeep := flag.Int("keep-logs", 7, "set number of logs to keep after rotation")
	mode := flag.String("mode", "naumen", "set program mode('csv' - use data/users.csv; 'naumen' - work with HD Naumen API & sqlite3 data/data.db &)")
	flag.Parse()

	// logging
	logFile, err := logging.StartLogging(appName, *logsDir, *logsToKeep)
	if err != nil {
		log.Fatalf("failed to start logging:\n\t%s", err)
	}

	defer logFile.Close()

	// starting programm notification
	startTime := time.Now()
	log.Println("Program Started")

	// TODO: refactor -> vafswork
	// READING FAZ DATA FILE
	fazDataFile, errFile := os.Open(fazDataFilePath)
	if errFile != nil {
		log.Fatal("FAILED: to open FAZ data file:\n\t", errFile)
	}
	defer fazDataFile.Close()

	byteFazData, errRead := io.ReadAll(fazDataFile)
	if errRead != nil {
		log.Fatalf("FAILED: to read FAZ data file:\n\t%v", errRead)
	}

	errJsonF := json.Unmarshal(byteFazData, &fazData)
	if errJsonF != nil {
		log.Fatalf("FAILED: to unmarshall FAZ data:\n\t%v", errJsonF)
	}

	// TODO: refactor -> vafswork
	// READING LDAP DATA FILE
	ldapDataFile, errFile := os.Open(ldapDataFilePath)
	if errFile != nil {
		log.Fatalf("FAILED: to open LDAP data file:\n\t%v", errFile)
	}
	defer fazDataFile.Close()

	byteLdapData, errRead := io.ReadAll(ldapDataFile)
	if errRead != nil {
		log.Fatalf("FAILED: to read LDAP data file:\n\t%v", errRead)
	}

	errJsonL := json.Unmarshal(byteLdapData, &ldapData)
	if errJsonL != nil {
		log.Fatalf("FAILED: to unmarshall LDAP data file:\n\t%v", errJsonL)
	}

	// CREATING REPORTS DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		log.Fatal("FAILED: to Create Reports Dir", err)
	}

	// different workflows for mode 'db'(default) & 'csv'
	switch *mode {
	case "naumen":
		// TODO: refactor -> vafswork
		// READING NAUMEN DATA FILE
		ldapDataFile, errFile := os.Open(naumenDataFilePath)
		if errFile != nil {
			log.Fatalf("FAILED: to open NAUMEN data file:\n\t%v", errFile)
		}
		defer fazDataFile.Close()

		byteLdapData, errRead := io.ReadAll(ldapDataFile)
		if errRead != nil {
			log.Fatalf("FAILED: to read NAUMEN data file:\n\t%v", errRead)
		}

		errJsonL := json.Unmarshal(byteLdapData, &naumenData)
		if errJsonL != nil {
			log.Fatalf("FAILED: to unmarshall NAUMEN data file:\n\t%v", errJsonL)
		}

		// getting list of unporcessed values in db
		unprocessedValues, err := dboperations.GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn)
		if err != nil {
			log.Fatalf("failed to get list of unprocessed values in db(%s):\n\t%v", dbFile, err)
		}
		// fmt.Println(unprocessedValues)

		// exit program if there are no values to process
		if len(unprocessedValues) == 0 {
			log.Fatalf("no values to process this time, exiting")
		}

		// loop to get all users & dates by DB unprocessedValues
		// TODO: consider goroutine
		for _, taskId := range unprocessedValues {
			// sumDescription, err := naumen.GetTaskSumDescriptionAndRP(naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, taskId)
			// if err != nil {
			// 	log.Fatalf("failed to do getData from Naumen for '%s':\n\t%v", taskId, err)
			// }
			// log.Printf("found sumDescription:\n\t%v\n", sumDescription)
			// fmt.Println(sumDescription)

			// TEST: sumDescription example:
			// "sumDescription": "<font color=\"#5f5f5f\">Укажите ФИО: <b>Surname1 Name1 Patronymic1, Surname2 Name2 Patronymic2</b>
			//   </font><br><font color=\"#5f5f5f\">Укажите дату:: <b>02.11.2024 00:01 - 03.11.2024 23:59</b></font><br>",
			sumDescription := []string{
				"RP2172655",
				"<font color=\"#5f5f5f\">Укажите ФИО: <b>Алексенцев Илья Константинович, Мамырбеков Данияр Хасенович, Марченко Максим Викторович</b></font><br><font color=\"#5f5f5f\">Укажите дату:: <b>02.11.2024 00:01 - 03.11.2024 23:59</b></font><br>",
			}

			// parse sumDescription for date
			// we need everyting between <b></b> after 'Укажите дату::'
			datesPattern := regexp.MustCompile(`.*?Укажите дату:+ +<b>(.*?)<\/b>.*`)
			// result will be in 1 index of FindStringSubmatch or 'nil' if not found
			datesSubexpr := datesPattern.FindStringSubmatch(sumDescription[1])
			if datesSubexpr == nil {
				log.Fatalf("failed to find dates subexpression in sumDescription of %s", taskId)
			}
			// next split subexpr for separate dates(start date then end date)
			datesFound := strings.Split(datesSubexpr[1], " - ")
			if len(datesFound) == 0 {
				log.Fatalf("no dates in result of usersSubexpr split!")
			}
			// next we need to format dates to FAZ format('00:00:01 2024/08/06')
			for ind, date := range datesFound {
				// convert string to time.Time(02.11.2024 00:01)
				tempDate, errT := time.Parse("02.01.2006 15:04", date)
				if errT != nil {
					log.Fatalf("failed to parse date string: %s", date)
				}
				// now format time to FAZ format
				datesFound[ind] = tempDate.Format("15:04:05 2006/01/02")
			}
			fmt.Println(datesFound)

			// parse sumDescription for users
			// we need everyting between <b></b> after 'Укажите ФИО:'
			usersNamesPattern := regexp.MustCompile(`.*?Укажите ФИО:+ +<b>(.*?)<\/b>.*`)
			// result will be in 1 index of FindStringSubmatch or 'nil' if not found
			usersSubexpr := usersNamesPattern.FindStringSubmatch(sumDescription[1])
			if usersSubexpr == nil {
				log.Fatalf("failed to find users subexpression in sumDescription of %s", taskId)
			}
			// next split subexpr for separate users
			usersFound := strings.Split(usersSubexpr[1], ",")
			if len(usersFound) == 0 {
				log.Fatalf("no users in result of usersSubexpr split!")
			}
			fmt.Println(usersFound)

			// forming users
			for _, foundUser := range usersFound {
				user.UserInitials = foundUser
				user.StartDate = datesFound[0]
				user.EndDate = datesFound[1]
				user.RP = sumDescription[0]
				user.DBId = taskId
				user.ServiceCall = sumDescription[0]
				// append formed user to users list
				users = append(users, user)
			}
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

	fmt.Println(users)
	os.Exit(0)

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
		adSearchResult, err := ldap.LdapBindAndSearch(user.Username, ldapData.LdapFqdn, ldapData.LdapBaseDn, ldapData.LdapBindUser, ldapData.LdapBindPass)
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
		// TODO: if 'naumen' save to RP/DBId/SC?
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

		// TODO: collect all reports by RP/DBId/ServiceCall
		// if mode 'naumen' - attach collected reports, close ticket(set wait for acceptance)
		if *mode == "naumen" {
			// TODO: take responsibility on request

			// TODO: attach file to RP and set 'wait for acceptance'

		}

		// TODO: update db value if success(change to 1 if success or 0 for failed)
		// errU := dboperations.UpdDbValue(dbFile, dbTable, dbValueColumn, dbProcessedColumn, dbProcessedDateColumn, "test7", 0)
		// if errU != nil {
		// 	log.Fatalf("failed to update value(%s) to result(%v):\n\t%v", "test7", 1, errU)
		// }
		// log.Printf("succeeded to update value(%s) to result(%v):\n", "test7", 1)
	}

	// count & print estimated time
	endTime := time.Now()
	log.Printf("Program Done\n\tEstimated time is %f seconds", endTime.Sub(startTime).Seconds())

	// close logfile and rotate logs
	logFile.Close()

	if err := vafswork.RotateFilesByMtime(*logsDir, *logsToKeep); err != nil {
		log.Fatalf("failed to rotate logs:\n\t%s", err)
	}
}
