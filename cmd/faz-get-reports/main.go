package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	fazrep "github.com/slayerjk/faz-get-reports/internal/fazrequests"
	"github.com/slayerjk/faz-get-reports/internal/helpers"
	models "github.com/slayerjk/faz-get-reports/internal/models"
	naumen "github.com/slayerjk/go-hd-naumen-api"
	mailing "github.com/slayerjk/go-mailing"
	vafswork "github.com/slayerjk/go-vafswork"
	ldap "github.com/slayerjk/go-valdapwork"
	vawebwork "github.com/slayerjk/go-vawebwork"
)

const (
	appName               = "faz-get-reports"
	dbTable               = "Data"
	dbValueColumn         = "Value"
	dbProcessedColumn     = "Processed"
	dbProcessedDateColumn = "Processed_Date"
)

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
	var (
		logsPath           = vafswork.GetExePath() + "/logs" + "_" + appName
		fazModelFilePath   = vafswork.GetExePath() + "/data/faz-data.json"
		ldapDataFilePath   = vafswork.GetExePath() + "/data/ldap-data.json"
		naumenDataFilePath = vafswork.GetExePath() + "/data/naumen-data.json"
		usersFilePath      = vafswork.GetExePath() + "/data/users.csv"
		resultsPath        = vafswork.GetExePath() + "/Reports"
		dbFile             = vafswork.GetExePath() + "/data/data.db"
		mailingFileDefault = vafswork.GetExePath() + "/data/mailing.json"
		mailErr            error
		ldapSamAccFilter   = "PAM-"

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

	fazModel := &fazrep.FazModelJson{}

	// flags
	logsDir := flag.String("log-dir", logsPath, "set custom log dir")
	logsToKeep := flag.Int("keep-logs", 7, "set number of logs to keep after rotation")
	mode := flag.String("mode", "naumen", "set program mode('csv' - use data/users.csv; 'naumen' - work with HD Naumen API & sqlite3 data/data.db &)")
	mailingOpt := flag.Bool("m", false, "turn the mailing options on(use 'data/mailing.json')")
	mailingFile := flag.String("mailing-file", mailingFileDefault, "full path to 'mailing.json'")
	hdSolutionText := flag.String("solution-text", "Запрос  исполнен, результат во вложении!", "set solution text for HD Request")
	dsn := flag.String("dsn", dbFile, "SQLITE3 db file full path")

	flag.Usage = func() {
		fmt.Println("Version: v0.2.0(21.02.2025)")
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	// logging
	// create log dir
	if err := os.MkdirAll(*logsDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stdout, "failed to create log dir %s:\n\t%v", *logsDir, err)
		os.Exit(1)
	}
	// set current date
	dateNow := time.Now().Format("02.01.2006")
	// create log file
	logFilePath := fmt.Sprintf("%s/%s_%s.log", *logsDir, appName, dateNow)
	// open log file in append mode
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stdout, "failed to open created log file %s:\n\t%v", logFilePath, err)
		os.Exit(1)
	}
	defer logFile.Close()
	// set logger
	logger := slog.New(slog.NewTextHandler(logFile, nil))

	// check if faz-get-report process is running already(exit if is already running)
	dublicateProcFound, err := helpers.IsAppAlreadyRunning(appName)
	if err != nil {
		logger.Error("failed to check if there are dublicate procs", slog.Any("ERR", err))
	}
	if dublicateProcFound {
		logger.Warn("application is already running, exiting this time")
		os.Exit(0)
	}

	// open db
	db, err := helpers.OpenDB(*dsn)
	if err != nil {
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte("failed to open DB file at openDB()"))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error("failed to open DB file", "DSN", *dsn, slog.Any("ERROR", err))
		os.Exit(1)
	}
	defer db.Close()

	// define db model instance
	dbModel := &models.DbModel{DB: db}

	// create map for Naumen RP data(RP, SC, files report)
	naumenSummary := make(map[string]map[string][]string)

	// starting programm notification
	startTime := time.Now()
	logger.Info("Program Started", "APP", appName, "MODE", *mode)

	// making http client for FAZ/HD Naumen request
	httpClient := vawebwork.NewInsecureClient()

	// READING FAZ DATA FILE
	fazModelFile, errFile := os.Open(fazModelFilePath)
	if errFile != nil {
		// report error
		errorDataFile := fmt.Sprintf("FAILURE: open FAZ data file:\n\t%v", errFile)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorDataFile))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorDataFile)
		os.Exit(1)
	}
	defer fazModelFile.Close()

	bytefazModel, errRead := io.ReadAll(fazModelFile)
	if errRead != nil {
		// report error
		errorfazModel := fmt.Sprintf("FAILURE: read FAZ data file:\n\t%v", errRead)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorfazModel))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorfazModel)
		os.Exit(1)
	}

	errJsonF := json.Unmarshal(bytefazModel, &fazModel)
	if errJsonF != nil {
		// report error
		errorfazModelJson := fmt.Sprintf("FAILURE: unmarshall FAZ data:\n\t%v", errJsonF)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorfazModelJson))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorfazModelJson)
		os.Exit(1)
	}

	// TODO: refactor -> vafswork
	// READING LDAP DATA FILE
	ldapDataFile, errFile := os.Open(ldapDataFilePath)
	if errFile != nil {
		// report error
		errorLdapData := fmt.Sprintf("FAILURE: open LDAP data file:\n\t%v", errFile)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorLdapData))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				os.Exit(1)
			}
		}
		logger.Error(errorLdapData)
		os.Exit(1)
	}
	defer fazModelFile.Close()

	byteLdapData, errRead := io.ReadAll(ldapDataFile)
	if errRead != nil {
		// report error
		errorLdapDataRead := fmt.Sprintf("FAILURE: read LDAP data file:\n\t%v", errRead)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorLdapDataRead))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorLdapDataRead)
		os.Exit(1)
	}

	errJsonL := json.Unmarshal(byteLdapData, &ldapData)
	if errJsonL != nil {
		// report error
		errorLdapDataJson := fmt.Sprintf("FAILURE: unmarshall LDAP data file:\n\t%v", errJsonL)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorLdapDataJson))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorLdapDataJson)
		os.Exit(1)
	}

	// CREATING REPORTS DIR IF NOT EXIST
	if err := os.MkdirAll(resultsPath, os.ModePerm); err != nil {
		// report error
		errorMkdirResults := fmt.Sprintf("FAILURE: create reports dir(%s):\n\t%v", resultsPath, err)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorMkdirResults))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorMkdirResults)
		os.Exit(1)
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
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorNaumenData))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorNaumenData)
			os.Exit(1)
		}
		defer naumenDataFile.Close()

		byteNaumenData, errRead := io.ReadAll(naumenDataFile)
		if errRead != nil {
			// report error
			errorNaumenDataRead := fmt.Sprintf("FAILURE: read NAUMEN data file:\n\t%v", errRead)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorNaumenDataRead))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorNaumenDataRead)
			os.Exit(1)
		}

		errJsonL := json.Unmarshal(byteNaumenData, &naumenData)
		if errJsonL != nil {
			// report error
			errorNaumenDataJson := fmt.Sprintf("FAILURE: unmarshall NAUMEN data file:\n\t%v", errJsonL)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorNaumenDataJson))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorNaumenDataJson)
			os.Exit(1)
		}

		// getting list of unporcessed values in db
		unprocessedValues, err := dbModel.GetUnprocessedDbValues(dbFile, dbTable, dbValueColumn, dbProcessedColumn)
		if err != nil {
			// report error
			errorUnprocessedValues := fmt.Sprintf("FAILURE: get list of unprocessed values in db(%s):\n\t%v", dbFile, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorUnprocessedValues))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorUnprocessedValues)
			os.Exit(1)
		}

		// exit program if there are no values to process
		if len(unprocessedValues) == 0 {
			logger.Warn("no values to process this time, exiting")
			os.Exit(1)
		}
		logger.Info("current unprocessed Naumen data ids", slog.Any("LIST", unprocessedValues))

		// loop to get all users & dates by DB unprocessedValues
		// TODO: consider goroutine
		for _, taskId := range unprocessedValues {
			sumDescription, err := naumen.GetTaskSumDescriptionAndRP(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, taskId)
			if err != nil {
				// report error
				errorSumDescription := fmt.Sprintf("FAILURE: get getData from Naumen for '%s':\n\t%v", taskId, err)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorSumDescription))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorSumDescription)
				os.Exit(1)
			}
			sumDescriptionFound := fmt.Sprintf("found sumDescription of %s(%s):\n\t%v\n", sumDescription[1], sumDescription[0], sumDescription[2])
			logger.Info(sumDescriptionFound)

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
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorDatesParsing))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorDatesParsing)
				os.Exit(1)
			}
			// next split subexpr for separate dates(start date then end date)
			datesFound := strings.Split(datesSubexpr[1], " - ")
			if len(datesFound) == 0 {
				// report error
				errorDatesEmpty := fmt.Sprintf("FAILURE: no dates in result of usersSubexpr split(%s)", taskId)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorDatesEmpty))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorDatesEmpty)
				os.Exit(1)
			}
			// next we need to format dates to FAZ format('00:00:01 2024/08/06')
			for ind, date := range datesFound {
				// convert string to time.Time(02.11.2024 00:01)
				tempDate, errT := time.Parse("02.01.2006 15:04", date)
				if errT != nil {
					// report error
					errorParseDateString := fmt.Sprintf("FAILURE: parse date string: %s(%s)", date, taskId)
					// mail this error if mailing option is on
					if *mailingOpt {
						mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorParseDateString))
						if mailErr != nil {
							logger.Warn("failed to send email", slog.Any("ERR", mailErr))
						}
					}
					logger.Error(errorParseDateString)
					os.Exit(1)
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
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorUsersParsing))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorUsersParsing)
				os.Exit(1)
			}
			// next split subexpr for separate users
			usersFound := strings.Split(usersSubexpr[1], ",")
			if len(usersFound) == 0 {
				// report error
				errorUsersEmpty := fmt.Sprintf("FAILURE: no users in result of usersSubexpr split(%s)!", taskId)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorUsersEmpty))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorUsersEmpty)
				os.Exit(1)
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
			errorCsvFile := fmt.Sprintf("FAILURE: open users file(%s):\n\t%v", usersFile.Name(), errFile)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorCsvFile))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorCsvFile)
			os.Exit(1)
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

	// GETTING FAZ SESSION ID
	logger.Info("getting FAZ session id")
	sessionid, errS := fazModel.GetSessionid(&httpClient, fazModel.FazUrl, fazModel.ApiUser, fazModel.ApiUserPass)
	if errS != nil {
		// report error
		errorFazSessionid := fmt.Sprintf("FAILURE: get FAZ sessionid\n\t%v", errS)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazSessionid))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorFazSessionid)
		os.Exit(1)
	}

	// GETTING FAZ REPORT LAYOUT
	fazReportLayout, errLayout := fazModel.GetFazReportLayout(&httpClient, fazModel.FazUrl, sessionid, fazModel.FazAdom, fazModel.FazReportName)
	if err != nil {
		// report error
		errorFazRepLayout := fmt.Sprintf("FAILURE: get FAZ report layout:\n\t%v", errLayout)
		// mail this error if mailing option is on
		if *mailingOpt {
			mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazRepLayout))
			if mailErr != nil {
				logger.Warn("failed to send email", slog.Any("ERR", mailErr))
			}
		}
		logger.Error(errorFazRepLayout)
		os.Exit(1)
	}

	// STARTING GETTING REPORT LOOP
	logger.Info("Users data to process in FAZ:")
	for _, user := range users {
		logger.Info("processing now", slog.Any("USR", user))
	}

	for _, user := range users {
		logger.Info("getting report job", "USR", user.Username)

		// GETTING AD user's samaccountname; exclude 'PAM-' accounts
		sAMAccountName, err = ldap.BindAndSearchSamaccountnameByDisplayname(
			user.Username,
			ldapData.LdapFqdn,
			ldapData.LdapBaseDn,
			ldapData.LdapBindUser,
			ldapData.LdapBindPass,
			ldapSamAccFilter,
		)
		if err != nil {
			// report error
			errorGetSamaccountName := fmt.Sprintf(
				"FAILURE: fetch AD samaccountname for '%s'(NaumenRP=%s):\n\t%v\n\tSkpping user",
				user.UserInitials,
				user.RP,
				err,
			)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorGetSamaccountName))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			// TODO: just skip user, don't shutdown the app
			// logger.Fatal(errorGetSamaccountName)
			logger.Warn(errorGetSamaccountName)
			continue
		}

		logger.Info("User's sAMAccountName found", "ACC", sAMAccountName)
		// os.Exit(0)

		// GETTING SESSIONID
		// report error
		// errorFazSessionid := fmt.Sprintf("FAILURE: get FAZ sessionid\n\t%v", errS)
		// mail this error if mailing option is on
		// if *mailingOpt {
		// 	mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazSessionid))
		// 	if mailErr != nil {
		// 		logger.Warn("failed to send email", slog.Any("ERR", mailErr))
		// 	}
		// }
		// logger.Fatal(errorFazSessionid)

		// UPDATING DATASETS QUERY
		errUpdDataset := fazModel.UpdateDatasets(&httpClient, fazModel.FazUrl, sessionid, fazModel.FazAdom, sAMAccountName, fazModel.FazDatasets)
		if errUpdDataset != nil {
			// report error
			errorfazModelsetUpd := fmt.Sprintf("FAILURE: to update FAZ datasets:\n\t%v", errUpdDataset)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorfazModelsetUpd))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorfazModelsetUpd)
			os.Exit(1)
		}

		// STARTING REPORT
		logger.Info("started running FAZ report job", "USR", user.Username)

		repId, err := fazModel.StartReport(&httpClient, fazModel.FazUrl, fazModel.FazAdom, fazModel.FazDevice, sessionid, user.StartDate, user.EndDate, fazReportLayout)
		if err != nil {
			// report error
			errorFazReportStart := fmt.Sprintf("FAILURE: to start FAZ report:\n\t%v", err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazReportStart))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorFazReportStart)
			os.Exit(1)
		}

		// DOWNLOADING PDF REPORT
		logger.Info("started downloading report", "USR", user.Username)

		repData, err := fazModel.DownloadPdfReport(&httpClient, fazModel.FazUrl, fazModel.FazAdom, sessionid, repId)
		if err != nil {
			// report error
			errorFazReportDownload := fmt.Sprintf("FAILURE: dowonload FAZ report:\n\t%v", err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazReportDownload))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorFazReportDownload)
			os.Exit(1)
		}

		// GETTING DATES FOR REPORT FILE
		tempTime, err := time.Parse("15:04:05 2006/01/02", user.StartDate)
		if err != nil {
			// report error
			errorUserStartTimeParse := fmt.Sprintf("FAILURE: to Parse User(%v) Start Time(%v):\n\t%v", user, tempTime, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorUserStartTimeParse))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorUserStartTimeParse)
			os.Exit(1)
		}
		repStartTime = tempTime.Format("02-01-2006-T-15-04-05")

		tempTime, err = time.Parse("15:04:05 2006/01/02", user.EndDate)
		if err != nil {
			// report error
			errorUserEndTimeParse := fmt.Sprintf("FAILURE: to Parse User(%v) End Time(%v):\n\t%v", user, tempTime, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorUserEndTimeParse))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorUserEndTimeParse)
			os.Exit(1)
		}
		repEndTime = tempTime.Format("02-01-2006-T-15-04-05")

		// SAVING REPORT TO FILE

		// decoding base64 data to []byte
		dec, err := base64.StdEncoding.DecodeString(repData)
		if err != nil {
			// report error
			errorFazReportDecode := fmt.Sprintf("FAILURE: to Decode Report Data(%s):\n\t%v", repData, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorFazReportDecode))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorFazReportDecode)
			os.Exit(1)
		}

		// forming report file full path
		reportFilePath = fmt.Sprintf("%s/%s_%s_%s.zip", resultsPath, user.UserInitials, repStartTime, repEndTime)
		// if mode == 'naumen' save to user.RP subdir of resultsPath
		if *mode == "naumen" {
			// creating Report dir for RP: 'Reports/RP***'
			if err := os.MkdirAll(resultsPath+"/"+user.RP, os.ModePerm); err != nil {
				// report error
				errorMkdirReportRP := fmt.Sprintf("FAILURE: create reports dir with RP(%s):\n\t%v", user.RP, err)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorMkdirReportRP))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorMkdirReportRP)
				os.Exit(1)
			}
			reportFilePath = fmt.Sprintf("%s/%s/%s.zip", resultsPath, user.RP, user.UserInitials)
		}

		// create empty report file(full path)
		file, err := os.Create(reportFilePath)
		if err != nil {
			// report error
			errorCreateReportBlankFile := fmt.Sprintf("FAILURE: to Create Report Blank File(%s):\n\t%v", reportFilePath, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorCreateReportBlankFile))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorCreateReportBlankFile)
			os.Exit(1)
		}
		defer file.Close()

		// write decoded data to report file
		if _, err := file.Write(dec); err != nil {
			// report error
			errorWriteReportData := fmt.Sprintf("FAILURE: to Write Report Data to File(%s):\n\t%v", reportFilePath, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorWriteReportData))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorWriteReportData)
			os.Exit(1)
		}
		if err := file.Sync(); err != nil {
			// report error
			errorSyncReportData := fmt.Sprintf("FAILURE: to Sync Written Report File(%s):\n\t%v", reportFilePath, err)
			// mail this error if mailing option is on
			if *mailingOpt {
				mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorSyncReportData))
				if mailErr != nil {
					logger.Warn("failed to send email", slog.Any("ERR", mailErr))
				}
			}
			logger.Error(errorSyncReportData)
			os.Exit(1)
		}

		// fill up summary for Naumen data with downloaded reports file pathes
		if *mode == "naumen" {
			naumenSummary[user.ServiceCall][user.RP] = append(naumenSummary[user.ServiceCall][user.RP], reportFilePath)
		}

		logger.Info("finished getting report job", "USR", user.Username, "RP", user.RP)
	}

	// if mode 'naumen' - attach collected reports, close ticket(set wait for acceptance)
	if *mode == "naumen" {
		logger.Info("Collected task data for Naumen RPs:")
		for sc, val := range naumenSummary {
			logger.Info("-", slog.Any("SC", sc), slog.Any("VAL", val))
		}

		// take responsibility on request, attach files and set acceptance
		for sc := range naumenSummary {
			// take responsibility on request
			logger.Info("started take responsibility on Naumen ticket", "SC", sc)

			errT := naumen.TakeSCResponsibility(&httpClient, naumenData.NaumenBaseUrl, naumenData.NaumenAccessKey, sc)
			if errT != nil {
				// report error
				errorTakeResp := fmt.Sprintf("FAILURE: take responsibility on Naumen ticket(%s):\n\t%v", naumenSummary[sc], errT)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorTakeResp))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Error(errorTakeResp)
				os.Exit(1)
			}

			// logger.Printf("FINISHED: take responsibility on Naumen ticket: %s\n", naumenSummary[sc])

			// attach files to RP and set acceptance
			for rp, files := range naumenSummary[sc] {
				logger.Info("started attaching files to ticket and set acceptance", "RP", rp)

				// for files skip 0 index, because it's dataID
				errA := naumen.AttachFilesAndSetAcceptance(
					&httpClient,
					naumenData.NaumenBaseUrl,
					naumenData.NaumenAccessKey,
					sc,
					*hdSolutionText,
					files[1:])
				if errA != nil {
					// report error
					errorAFSA := fmt.Sprintf("FAILURE: attaching files to ticket and set acceptance(%s):\n\t%v", rp, errA)
					// mail this error if mailing option is on
					if *mailingOpt {
						mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorAFSA))
						if mailErr != nil {
							logger.Warn("failed to send email", slog.Any("ERR", mailErr))
						}
					}
					logger.Error(errorAFSA)
					os.Exit(1)
				}

				logger.Info("finished take responsibility, attach reports and set acceptance on Naumen ticket", "RP", rp)

				// TODO: update db value if success(change to 1 if success or 0 for failure)
				logger.Info("started update db with success result", "VAL", naumenSummary[sc][rp][0])

				errU := dbModel.UpdDbValue(
					dbFile, dbTable, dbValueColumn, dbProcessedColumn, dbProcessedDateColumn,
					naumenSummary[sc][rp][0], 1)
				if errU != nil {
					// report error
					errorDbUpd := fmt.Sprintf("FAILURE: update value(%s) to result(%v):\n\t%v", naumenSummary[sc][rp][0], 1, errU)
					// mail this error if mailing option is on
					if *mailingOpt {
						mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "ERR", appName, []byte(errorDbUpd))
						if mailErr != nil {
							logger.Warn("failed to send email", slog.Any("ERR", mailErr))
						}
					}
					logger.Error(errorDbUpd)
					os.Exit(1)
				}

				// report success
				reportDbUPD := fmt.Sprintf("FINISHED: processing, including DBUpd: %s\n", rp)
				// mail this error if mailing option is on
				if *mailingOpt {
					mailErr = mailing.SendPlainEmailWoAuth(*mailingFile, "report", appName, []byte(reportDbUPD))
					if mailErr != nil {
						logger.Warn("failed to send email", slog.Any("ERR", mailErr))
					}
				}
				logger.Info(reportDbUPD)
			}
		}
	}

	// count & print estimated time
	endTime := time.Now()
	logger.Info("Program's job is Done", slog.Any("estimated time(sec)", endTime.Sub(startTime).Seconds()))

	// close logfile and rotate logs
	logFile.Close()

	if err := vafswork.RotateFilesByMtime(*logsDir, *logsToKeep); err != nil {
		fmt.Fprintf(os.Stdout, "failure to rotate logs:\n\t%s", err)
	}
}
