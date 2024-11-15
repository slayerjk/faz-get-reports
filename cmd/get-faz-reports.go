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
	"github.com/slayerjk/faz-get-reports/internal/mailing"
	"github.com/slayerjk/faz-get-reports/internal/vafswork"
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
		mailingFile        = vafswork.GetExePath() + "/data/mailing.json"

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
		// report error
		errorLogging := fmt.Sprintf("FAILURE: start logging:\n\t%s", err)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorLogging))
		log.Fatal(errorLogging)
	}

	defer logFile.Close()

	// starting programm notification
	startTime := time.Now()
	log.Println("Program Started")

	// TODO: refactor -> vafswork
	// READING FAZ DATA FILE
	fazDataFile, errFile := os.Open(fazDataFilePath)
	if errFile != nil {
		// report error
		errorDataFile := fmt.Sprintf("FAILURE: open FAZ data file:\n\t", errFile)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorDataFile))
		log.Fatal(errorDataFile)
	}
	defer fazDataFile.Close()

	byteFazData, errRead := io.ReadAll(fazDataFile)
	if errRead != nil {
		// report error
		errorFazData := fmt.Sprintf("FAILURE: read FAZ data file:\n\t%v", errRead)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazData))
		log.Fatal(errorFazData)
	}

	errJsonF := json.Unmarshal(byteFazData, &fazData)
	if errJsonF != nil {
		// report error
		errorFazDataJson := fmt.Sprintf("FAILURE: unmarshall FAZ data:\n\t%v", errJsonF)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazDataJson))
		log.Fatal(errorFazDataJson)
	}

	// TODO: refactor -> vafswork
	// READING LDAP DATA FILE
	ldapDataFile, errFile := os.Open(ldapDataFilePath)
	if errFile != nil {
		// report error
		errorLdapData := fmt.Sprintf("FAILURE: open LDAP data file:\n\t%v", errFile)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorLdapData))
		log.Fatal(errorLdapData)
	}
	defer fazDataFile.Close()

	byteLdapData, errRead := io.ReadAll(ldapDataFile)
	if errRead != nil {
		// report error
		errorLdapDataRead := fmt.Sprintf("FAILURE: read LDAP data file:\n\t%v", errRead)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorLdapDataRead))
		log.Fatal(errorLdapDataRead)
	}

	errJsonL := json.Unmarshal(byteLdapData, &ldapData)
	if errJsonL != nil {
		// report error
		errorLdapDataJson := fmt.Sprintf("FAILURE: unmarshall LDAP data file:\n\t%v", errJsonL)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorLdapDataJson))
		log.Fatal(errorLdapDataJson)
	}

	// CREATING REPORTS DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		// report error
		errorMkdirResults := fmt.Sprintf("FAILURE: create reports dir", err)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorMkdirResults))
		log.Fatal(errorMkdirResults)
	}

	// different workflows for mode 'db'(default) & 'csv'
	switch *mode {
	case "naumen":
		// TODO: refactor -> vafswork
		// READING NAUMEN DATA FILE
		naumenDataFile, errFile := os.Open(naumenDataFilePath)
		if errFile != nil {
			// report error
			errorNaumenData := fmt.Sprintf("FAILURE: open NAUMEN data file:\n\t%v", errFile)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorNaumenData))
			log.Fatal(errorNaumenData)
		}
		defer naumenDataFile.Close()

		byteNaumenData, errRead := io.ReadAll(ldapDataFile)
		if errRead != nil {
			// report error
			errorNaumenDataRead := fmt.Sprintf("FAILURE: read NAUMEN data file:\n\t%v", errRead)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorNaumenDataRead))
			log.Fatal(errorNaumenDataRead)
		}

		errJsonL := json.Unmarshal(byteNaumenData, &naumenData)
		if errJsonL != nil {
			// report error
			errorNaumenDataJson := fmt.Sprintf("FAILURE: unmarshall NAUMEN data file:\n\t%v", errJsonL)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorNaumenDataJson))
			log.Fatal(errorNaumenDataJson)
		}

		// getting list of unporcessed values in db
		unprocessedValues, err := dboperations.GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn)
		if err != nil {
			// report error
			errorUnprocessedValues := fmt.Sprintf("FAILURE: get list of unprocessed values in db(%s):\n\t%v", dbFile, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorUnprocessedValues))
			log.Fatal(errorUnprocessedValues)
		}
		// fmt.Println(unprocessedValues)

		// exit program if there are no values to process
		if len(unprocessedValues) == 0 {
			// mailing.SendPlainEmailWoAuth(mailingFile, "report", appName, []byte("INFO: no values to process this time, exiting"))
			log.Fatalf("INFO: no values to process this time, exiting")
		}

		// loop to get all users & dates by DB unprocessedValues
		// TODO: consider goroutine
		for _, taskId := range unprocessedValues {
			httpClient := naumen.NewApiInsecureClient()

			sumDescription, err := naumen.GetTaskSumDescriptionAndRP(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, taskId)
			if err != nil {
				// report error
				errorSumDescription := fmt.Sprintf("FAILURE: get getData from Naumen for '%s':\n\t%v", taskId, err)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorSumDescription))
				log.Fatal(errorSumDescription)
			}
			log.Printf("found sumDescription of %s(%s):\n\t%v\n", sumDescription[1], sumDescription[0], sumDescription[2])

			// sumDescription example:
			// "sumDescription": "<font color=\"#5f5f5f\">Укажите ФИО: <b>Surname1 Name1 Patronymic1, Surname2 Name2 Patronymic2</b>
			//   </font><br><font color=\"#5f5f5f\">Укажите дату:: <b>02.11.2024 00:01 - 03.11.2024 23:59</b></font><br>",

			// parse sumDescription for date
			// we need everyting between <b></b> after 'Укажите дату::'
			datesPattern := regexp.MustCompile(`.*?Укажите дату:+ +<b>(.*?)<\/b>.*`)
			// result will be in 2 index of FindStringSubmatch or 'nil' if not found
			datesSubexpr := datesPattern.FindStringSubmatch(sumDescription[2])
			if datesSubexpr == nil {
				// report error
				errorDatesParsing := fmt.Sprintf("FAILURE: find dates subexpression in sumDescription of %s", taskId)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorDatesParsing))
				log.Fatal(errorDatesParsing)
			}
			// next split subexpr for separate dates(start date then end date)
			datesFound := strings.Split(datesSubexpr[1], " - ")
			if len(datesFound) == 0 {
				// report error
				errorDatesEmpty := fmt.Sprintf("FAILURE: no dates in result of usersSubexpr split(%s)", taskId)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorDatesEmpty))
				log.Fatal(errorDatesEmpty)
			}
			// next we need to format dates to FAZ format('00:00:01 2024/08/06')
			for ind, date := range datesFound {
				// convert string to time.Time(02.11.2024 00:01)
				tempDate, errT := time.Parse("02.01.2006 15:04", date)
				if errT != nil {
					// report error
					errorParseDateString := fmt.Sprintf("FAILURE: parse date string: %s(%s)", date, taskId)
					mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorParseDateString))
					log.Fatal(errorParseDateString)
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
				// report error
				errorUsersParsing := fmt.Sprintf("FAILURE: find users subexpression in sumDescription of %s", taskId)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorUsersParsing))
				log.Fatal(errorUsersParsing)
			}
			// next split subexpr for separate users
			usersFound := strings.Split(usersSubexpr[1], ",")
			if len(usersFound) == 0 {
				// report error
				errorUsersEmpty := fmt.Sprintf("FAILURE: no users in result of usersSubexpr split(%s)!", taskId)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorUsersEmpty))
				log.Fatal(errorUsersEmpty)
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
				// EXAMPLE:
				// <SCID, ex.: serviceCall$725912253>:
				//	map[<RPID: ex, RP2172655>:
				//	[<FIRST ELEM iS DATAID, ex: data$3242604> <OTHER ELEM WILL BE DOWNLOADED REPORTS FILE PATHES>
				naumenSummary[user.ServiceCall] = map[string][]string{user.RP: {taskId}}
			}
		}
	case "csv":
		// READING USERS FILE
		usersFile, errFile := os.Open(usersFilePath)

		if errFile != nil {
			// report error
			errorCsvFile := fmt.Sprintf("FAILURE: open users file(%s):\n\t", usersFile, errFile)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorCsvFile))
			log.Fatal(errorCsvFile)
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
		// report error
		errorFazSessionid := fmt.Sprintf("FAILURE: get FAZ sessionid\n\t%v", errS)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazSessionid))
		log.Fatal(errorFazSessionid)
	}

	fazReportLayout, errLayout := fazrep.GetFazReportLayout(fazData.FazUrl, sessionid, fazData.FazAdom, fazData.FazReportName)
	if err != nil {
		// report error
		errorFazRepLayout := fmt.Sprintf("FAILURE: get FAZ report layout:\n\t%v", errLayout)
		mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazRepLayout))
		log.Fatal(errorFazRepLayout)
	}

	// STARTING GETTING REPORT LOOP
	log.Println("Users data to process in FAZ:")
	for _, user := range users {
		log.Printf("\t%v\n", user)
	}

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
			// report error
			errorGetSamaccountName := fmt.Sprintf("FAILURE: fetch AD samaccountname for '%s':\n\t%v", user.UserInitials, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorGetSamaccountName))
			log.Fatal(errorGetSamaccountName)
		}

		log.Printf("User's sAMAccountName found: %s", sAMAccountName)

		// GETTING SESSIONID
		// report error
		// errorFazSessionid := fmt.Sprintf("FAILURE: get FAZ sessionid\n\t%v", errS)
		// mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazSessionid))
		// log.Fatal(errorFazSessionid)

		// UPDATING DATASETS QUERY
		errUpdDataset := fazrep.UpdateDatasets(fazData.FazUrl, sessionid, fazData.FazAdom, sAMAccountName, fazData.FazDatasets)
		if errUpdDataset != nil {
			// report error
			errorFazDatasetUpd := fmt.Sprintf("FAILURE: to update FAZ datasets:\n\t", errUpdDataset)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazDatasetUpd))
			log.Fatal(errorFazDatasetUpd)
		}

		// STARTING REPORT
		log.Printf("STARTED: running FAZ report job: %s\n", user.Username)

		repId, err := fazrep.StartReport(fazData.FazUrl, fazData.FazAdom, fazData.FazDevice, sessionid, user.StartDate, user.EndDate, fazReportLayout)
		if err != nil {
			// report error
			errorFazReportStart := fmt.Sprintf("FAILURE: to start FAZ report:\n\t%v", err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazReportStart))
			log.Fatal(errorFazReportStart)
		}

		// DOWNLOADING PDF REPORT
		log.Printf("STARTED: downloading for %s\n", user.Username)

		repData, err := fazrep.DownloadPdfReport(fazData.FazUrl, fazData.FazAdom, sessionid, repId)
		if err != nil {
			// report error
			errorFazReportDownload := fmt.Sprintf("FAILURE: dowonload FAZ report:\n\t%v", err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazReportDownload))
			log.Fatal(errorFazReportDownload)
		}

		// GETTING DATES FOR REPORT FILE
		tempTime, err := time.Parse("15:04:05 2006/01/02", user.StartDate)
		if err != nil {
			// report error
			errorUserStartTimeParse := fmt.Sprintf("FAILURE: to Parse User(%v) Start Time(%v):\n\t%v", user, tempTime, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorUserStartTimeParse))
			log.Fatal(errorUserStartTimeParse)
		}
		repStartTime = tempTime.Format("02-01-2006-T-15-04-05")

		tempTime, err = time.Parse("15:04:05 2006/01/02", user.EndDate)
		if err != nil {
			// report error
			errorUserEndTimeParse := fmt.Sprintf("FAILURE: to Parse User(%v) End Time(%v):\n\t%v", user, tempTime, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorUserEndTimeParse))
			log.Fatal(errorUserEndTimeParse)
		}
		repEndTime = tempTime.Format("02-01-2006-T-15-04-05")

		// SAVING REPORT TO FILE

		// decoding base64 data to []byte
		dec, err := base64.StdEncoding.DecodeString(repData)
		if err != nil {
			// report error
			errorFazReportDecode := fmt.Sprintf("FAILURE: to Decode Report Data(%s):\n\t%v", repData, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorFazReportDecode))
			log.Fatal(errorFazReportDecode)
		}

		// forming report file full path
		reportFilePath = fmt.Sprintf("%s/%s_%s_%s.zip", resultsPath, user.UserInitials, repStartTime, repEndTime)
		// if mode == 'naumen' save to user.RP subdir of resultsPath
		if *mode == "naumen" {
			// creating Report dir for RP: 'Reports/RP***'
			if err := os.MkdirAll(resultsPath+"/"+user.RP, os.ModePerm); err != nil {
				// report error
				errorMkdirReportRP := fmt.Sprintf("FAILURE: create reports dir with RP(%s):\n\t%v", user.RP, err)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorMkdirReportRP))
				log.Fatal(errorMkdirReportRP)
			}
			reportFilePath = fmt.Sprintf("%s/%s/%s.zip", resultsPath, user.RP, user.UserInitials)
		}

		// create empty report file(full path)
		file, err := os.Create(reportFilePath)
		if err != nil {
			// report error
			errorCreateReportBlankFile := fmt.Sprintf("FAILURE: to Create Report Blank File(%s):\n\t%v", reportFilePath, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorCreateReportBlankFile))
			log.Fatal(errorCreateReportBlankFile)
		}
		defer file.Close()

		// write decoded data to report file
		if _, err := file.Write(dec); err != nil {
			// report error
			errorWriteReportData := fmt.Sprintf("FAILURE: to Write Report Data to File(%s):\n\t%v", reportFilePath, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorWriteReportData))
			log.Fatal(errorWriteReportData)
		}
		if err := file.Sync(); err != nil {
			// report error
			errorSyncReportData := fmt.Sprintf("FAILURE: to Sync Written Report File(%s):\n\t%v", reportFilePath, err)
			mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorSyncReportData))
			log.Fatal(errorSyncReportData)
		}

		// fill up summary for Naumen data with downloaded reports file pathes
		naumenSummary[user.ServiceCall][user.RP] = append(naumenSummary[user.ServiceCall][user.RP], reportFilePath)

		log.Printf("FINISHED: GETTING REPORT JOB: %s(Naumen RP = %s)\n\n", user.Username, user.RP)
	}

	// if mode 'naumen' - attach collected reports, close ticket(set wait for acceptance)
	if *mode == "naumen" {
		log.Println("Collected task data for Naumen RP:")
		for sc, val := range naumenSummary {
			log.Printf("\t%v: %v\n", sc, val)
		}

		// take responsibility on request, attach files and set acceptance
		for sc := range naumenSummary {
			// making http client
			httpClient := naumen.NewApiInsecureClient()

			// take responsibility on request
			log.Printf("STARTED: take responsibility on Naumen ticket: %s", sc)

			errT := naumen.TakeSCResponsibility(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, sc)
			if errT != nil {
				// report error
				errorTakeResp := fmt.Sprintf("FAILURE: take responsibility on Naumen ticket(%s):\n\t%v", naumenSummary[sc], errT)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorTakeResp))
				log.Fatal(errorTakeResp)
			}

			// log.Printf("FINISHED: take responsibility on Naumen ticket: %s\n", naumenSummary[sc])

			// attach files to RP and set acceptance
			for rp, files := range naumenSummary[sc] {
				log.Printf("STARTED: attaching files to ticket and set acceptance(%s)", rp)

				// for files skip 0 index, because it's dataID
				errA := naumen.AttachFilesAndSetAcceptance(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, sc, files[1:])
				if errA != nil {
					// report error
					errorAFSA := fmt.Sprintf("FAILURE: attaching files to ticket and set acceptance(%s):\n\t%v", rp, errA)
					mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorAFSA))
					log.Fatal(errorAFSA)
				}

				log.Printf("FINISHED: take responsibility, attach reports and set acceptance on Naumen ticket: %s\n", rp)

				// TODO: update db value if success(change to 1 if success or 0 for failure)
				log.Printf("STARTED: update db with success result for value: %s", naumenSummary[sc][rp][0])

				errU := dboperations.UpdDbValue(
					dbFile, dbTable, dbValueColumn, dbProcessedColumn, dbProcessedDateColumn,
					naumenSummary[sc][rp][0], 1)
				if errU != nil {
					// report error
					errorDbUpd := fmt.Sprintf("FAILURE: update value(%s) to result(%v):\n\t%v", naumenSummary[sc][rp][0], 1, errU)
					mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(errorDbUpd))
					log.Fatal(errorDbUpd)
				}

				// report success
				reportDbUPD := fmt.Sprintf("FINISHED: processing, including DBUpd: %s\n", rp)
				mailing.SendPlainEmailWoAuth(mailingFile, "error", appName, []byte(reportDbUPD))
				log.Println(reportDbUPD)
			}
		}
	}

	// count & print estimated time
	endTime := time.Now()
	log.Printf("Program Done\n\tEstimated time is %f seconds", endTime.Sub(startTime).Seconds())

	// close logfile and rotate logs
	logFile.Close()

	if err := vafswork.RotateFilesByMtime(*logsDir, *logsToKeep); err != nil {
		log.Fatalf("failure to rotate logs:\n\t%s", err)
	}
}
