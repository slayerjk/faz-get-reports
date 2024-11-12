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
	naumen "github.com/slayerjk/faz-get-reports/internal/hd-naumen-api"
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

// struct of Naumen RP summary
type NaumenRPSummary map[string]map[string][]string

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

	// create map for Naumen RP data(RP, SC, files report)
	naumenSummary := make(map[string]map[string][]string)

	// flags
	logsDir := flag.String("log-dir", logsPath, "set custom log dir")
	logsToKeep := flag.Int("keep-logs", 7, "set number of logs to keep after rotation")
	mode := flag.String("mode", "naumen", "set program mode('csv' - use data/users.csv; 'naumen' - work with HD Naumen API & sqlite3 data/data.db &)")
	flag.Parse()

	// logging
	logFile, err := logging.StartLogging(appName, *logsDir, *logsToKeep)
	if err != nil {
		log.Fatalf("FAILURE: start logging:\n\t%s", err)
	}

	defer logFile.Close()

	// starting programm notification
	startTime := time.Now()
	log.Println("Program Started")

	// TODO: refactor -> vafswork
	// READING FAZ DATA FILE
	fazDataFile, errFile := os.Open(fazDataFilePath)
	if errFile != nil {
		log.Fatal("FAILURE: open FAZ data file:\n\t", errFile)
	}
	defer fazDataFile.Close()

	byteFazData, errRead := io.ReadAll(fazDataFile)
	if errRead != nil {
		log.Fatalf("FAILURE: read FAZ data file:\n\t%v", errRead)
	}

	errJsonF := json.Unmarshal(byteFazData, &fazData)
	if errJsonF != nil {
		log.Fatalf("FAILURE: unmarshall FAZ data:\n\t%v", errJsonF)
	}

	// TODO: refactor -> vafswork
	// READING LDAP DATA FILE
	ldapDataFile, errFile := os.Open(ldapDataFilePath)
	if errFile != nil {
		log.Fatalf("FAILURE: open LDAP data file:\n\t%v", errFile)
	}
	defer fazDataFile.Close()

	byteLdapData, errRead := io.ReadAll(ldapDataFile)
	if errRead != nil {
		log.Fatalf("FAILURE: read LDAP data file:\n\t%v", errRead)
	}

	errJsonL := json.Unmarshal(byteLdapData, &ldapData)
	if errJsonL != nil {
		log.Fatalf("FAILURE: unmarshall LDAP data file:\n\t%v", errJsonL)
	}

	// CREATING REPORTS DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		log.Fatal("FAILURE: create reports dir", err)
	}

	// different workflows for mode 'db'(default) & 'csv'
	switch *mode {
	case "naumen":
		// TODO: refactor -> vafswork
		// READING NAUMEN DATA FILE
		ldapDataFile, errFile := os.Open(naumenDataFilePath)
		if errFile != nil {
			log.Fatalf("FAILURE: open NAUMEN data file:\n\t%v", errFile)
		}
		defer fazDataFile.Close()

		byteLdapData, errRead := io.ReadAll(ldapDataFile)
		if errRead != nil {
			log.Fatalf("FAILURE: read NAUMEN data file:\n\t%v", errRead)
		}

		errJsonL := json.Unmarshal(byteLdapData, &naumenData)
		if errJsonL != nil {
			log.Fatalf("FAILURE: unmarshall NAUMEN data file:\n\t%v", errJsonL)
		}

		// getting list of unporcessed values in db
		unprocessedValues, err := dboperations.GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn)
		if err != nil {
			log.Fatalf("FAILURE: get list of unprocessed values in db(%s):\n\t%v", dbFile, err)
		}
		// fmt.Println(unprocessedValues)

		// exit program if there are no values to process
		if len(unprocessedValues) == 0 {
			log.Fatalf("FAILURE: no values to process this time, exiting")
		}

		// loop to get all users & dates by DB unprocessedValues
		// TODO: consider goroutine
		for _, taskId := range unprocessedValues {
			httpClient := naumen.NewApiInsecureClient()

			sumDescription, err := naumen.GetTaskSumDescriptionAndRP(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, taskId)
			if err != nil {
				log.Fatalf("FAILURE: get getData from Naumen for '%s':\n\t%v", taskId, err)
			}
			log.Printf("found sumDescription of %s(%s):\n\t%v\n", sumDescription[1], sumDescription[0], sumDescription[2])
			// fmt.Println(sumDescription)

			// sumDescription example:
			// "sumDescription": "<font color=\"#5f5f5f\">Укажите ФИО: <b>Surname1 Name1 Patronymic1, Surname2 Name2 Patronymic2</b>
			//   </font><br><font color=\"#5f5f5f\">Укажите дату:: <b>02.11.2024 00:01 - 03.11.2024 23:59</b></font><br>",

			// parse sumDescription for date
			// we need everyting between <b></b> after 'Укажите дату::'
			datesPattern := regexp.MustCompile(`.*?Укажите дату:+ +<b>(.*?)<\/b>.*`)
			// result will be in 2 index of FindStringSubmatch or 'nil' if not found
			datesSubexpr := datesPattern.FindStringSubmatch(sumDescription[2])
			if datesSubexpr == nil {
				log.Fatalf("FAILURE: find dates subexpression in sumDescription of %s", taskId)
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
					log.Fatalf("FAILURE: parse date string: %s", date)
				}
				// now format time to FAZ format
				datesFound[ind] = tempDate.Format("15:04:05 2006/01/02")
			}
			// fmt.Println(datesFound)

			// parse sumDescription for users
			// we need everyting between <b></b> after 'Укажите ФИО:'
			usersNamesPattern := regexp.MustCompile(`.*?Укажите ФИО:+ +<b>(.*?)<\/b>.*`)
			// result will be in 2 index of FindStringSubmatch or 'nil' if not found
			usersSubexpr := usersNamesPattern.FindStringSubmatch(sumDescription[2])
			if usersSubexpr == nil {
				log.Fatalf("FAILURE: find users subexpression in sumDescription of %s", taskId)
			}
			// next split subexpr for separate users
			usersFound := strings.Split(usersSubexpr[1], ",")
			if len(usersFound) == 0 {
				log.Fatalf("no users in result of usersSubexpr split!")
			}
			// fmt.Println(usersFound)

			// forming users
			for _, foundUser := range usersFound {
				user.Username = strings.Trim(foundUser, " ")
				user.StartDate = datesFound[0]
				user.EndDate = datesFound[1]
				user.RP = sumDescription[1]
				user.DBId = taskId
				user.ServiceCall = sumDescription[0]

				// GETTING USER INITIALS
				tempList = []string{}
				for ind, item := range strings.Split(user.Username, " ") {
					if ind == 0 {
						tempList = append(tempList, item)
					} else {
						runeItem := []rune(item)
						tempList = append(tempList, fmt.Sprintf("%s.", string(runeItem[0:1])))
					}
				}
				user.UserInitials = strings.Join(tempList, " ")

				// append formed user to users list
				users = append(users, user)

				// fill up summary for Naumen data
				naumenSummary[user.RP] = map[string][]string{user.ServiceCall: {}}
			}
		}
	case "csv":
		// READING USERS FILE
		usersFile, errFile := os.Open(usersFilePath)

		if errFile != nil {
			log.Fatal("FAILURE: open users file:\n\t", errFile)
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

	//fmt.Printf("%v", users)

	// GETTING FAZ REPORT LAYOUT
	sessionid, errS := fazrep.GetSessionid(fazData.FazUrl, fazData.ApiUser, fazData.ApiUserPass)
	if errS != nil {
		log.Fatal("FAILURE: get sessionid\n\t", errS)
	}

	fazReportLayout, errLayout := fazrep.GetFazReportLayout(fazData.FazUrl, sessionid, fazData.FazAdom, fazData.FazReportName)
	if err != nil {
		log.Fatalf("FAILURE: get report layout:\n\t%v", errLayout)
	}

	// STARTING GETTING REPORT LOOP
	log.Printf("Users data to process in FAZ:\n\t%+v", users)

	for _, user := range users {
		log.Printf("STARTED: GETTING REPORT JOB: %s\n", user.Username)

		// GETTING AD user's samaccountname
		sAMAccountName, err = ldap.BindAndSearchSamaccountnameByDisplayname(
			user.Username,
			ldapData.LdapFqdn,
			ldapData.LdapBaseDn,
			ldapData.LdapBindUser,
			ldapData.LdapBindPass,
		)
		if err != nil {
			log.Fatalf("FAILURE: fetch AD samaccountname for '%s':\n\t%v", user.UserInitials, err)
		}

		log.Printf("User's sAMAccountName found: %s", sAMAccountName)

		// GETTING SESSIONID
		sessionid, err := fazrep.GetSessionid(fazData.FazUrl, fazData.ApiUser, fazData.ApiUserPass)
		if err != nil {
			log.Fatal("FAILURE: to get sessionid\n\t", err)
		}

		// UPDATING DATASETS QUERIE
		errUpdDataset := fazrep.UpdateDatasets(fazData.FazUrl, sessionid, fazData.FazAdom, sAMAccountName, fazData.FazDatasets)
		if errUpdDataset != nil {
			log.Fatal("FAILURE: to update datasets:\n\t", errUpdDataset)
		}

		// STARTING REPORT
		log.Printf("STARTED: running report job: %s\n", user.Username)

		repId, err := fazrep.StartReport(fazData.FazUrl, fazData.FazAdom, fazData.FazDevice, sessionid, user.StartDate, user.EndDate, fazReportLayout)
		if err != nil {
			log.Fatal("FAILURE: to start report", err)
		}

		// DOWNLOADING PDF REPORT
		log.Printf("STARTED: downloading for %s\n", user.Username)

		repData, err := fazrep.DownloadPdfReport(fazData.FazUrl, fazData.FazAdom, sessionid, repId)
		if err != nil {
			log.Fatal("FAILURE: to dowonload report:\n\t", err)
		}

		// GETTING DATES FOR REPORT FILE
		tempTime, err := time.Parse("15:04:05 2006/01/02", user.StartDate)
		if err != nil {
			log.Fatal("FAILURE: to Parse User Start Time:\n\t", err)
		}
		repStartTime = tempTime.Format("02-01-2006-T-15-04-05")

		tempTime, err = time.Parse("15:04:05 2006/01/02", user.EndDate)
		if err != nil {
			log.Fatal("FAILURE: to Parse User End Time:\n\t", err)
		}
		repEndTime = tempTime.Format("02-01-2006-T-15-04-05")

		// SAVING REPORT TO FILE

		// decoding base64 data to []byte
		dec, err := base64.StdEncoding.DecodeString(repData)
		if err != nil {
			log.Fatal("FAILURE: to Decode Report Data:\n\t", err)
		}

		// forming report file full path
		reportFilePath = fmt.Sprintf("%s/%s_%s_%s.zip", resultsPath, user.UserInitials, repStartTime, repEndTime)
		// if mode == 'naumen' save to user.RP subdir of resultsPath
		if *mode == "naumen" {
			// creating Report dir for RP: 'Reports/RP***'
			if err := os.MkdirAll(resultsPath+"/"+user.RP, os.ModePerm); err != nil {
				log.Fatal("FAILURE: create reports dir with RP", err)
			}
			reportFilePath = fmt.Sprintf("%s/%s/%s.zip", resultsPath, user.RP, user.UserInitials)
		}

		// create empty report file(full path)
		file, err := os.Create(reportFilePath)
		if err != nil {
			log.Fatal("FAILURE: to Create Report Blank File:\n\t", err)
		}
		defer file.Close()

		// write decoded data to report file
		if _, err := file.Write(dec); err != nil {
			log.Fatal("FAILURE: to Write Report Data to File:\n\t", err)
		}
		if err := file.Sync(); err != nil {
			log.Fatal("FAILURE: to Sync Written Report File:\n\t", err)
		}

		// fill up summary for Naumen data
		naumenSummary[user.RP][user.ServiceCall] = append(naumenSummary[user.RP][user.ServiceCall], reportFilePath)

		log.Printf("FINISHED: GETTING REPORT JOB: %s(Naumen RP = %s)\n\n", user.Username, user.RP)
	}

	// if mode 'naumen' - attach collected reports, close ticket(set wait for acceptance)
	if *mode == "naumen" {
		fmt.Println(naumenSummary)
		log.Printf("Collected task data for Naumen RP:\n\t%+v\n", naumenSummary)
		os.Exit(0)
		// TODO: take responsibility on request
		for rp, _ := range naumenSummary {
			for sc, _ := range naumenSummary[rp] {
				// TODO: take responsibility on Naumen task(takeSCResponsibility)
				fmt.Println(sc)
			}
		}
		// TODO: attach file to RP and set 'wait for acceptance'

	}

	// TODO: update db value if success(change to 1 if success or 0 for failure)
	// errU := dboperations.UpdDbValue(dbFile, dbTable, dbValueColumn, dbProcessedColumn, dbProcessedDateColumn, "test7", 0)
	// if errU != nil {
	// 	log.Fatalf("failure to update value(%s) to result(%v):\n\t%v", "test7", 1, errU)
	// }
	// log.Printf("succeeded to update value(%s) to result(%v):\n", "test7", 1)

	// count & print estimated time
	endTime := time.Now()
	log.Printf("Program Done\n\tEstimated time is %f seconds", endTime.Sub(startTime).Seconds())

	// close logfile and rotate logs
	logFile.Close()

	if err := vafswork.RotateFilesByMtime(*logsDir, *logsToKeep); err != nil {
		log.Fatalf("failure to rotate logs:\n\t%s", err)
	}
}
