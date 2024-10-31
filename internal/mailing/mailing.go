package mailing

import (
	"encoding/json"
	"fmt"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type MailData struct {
	Host          string   `json:"host"`
	Port          string   `json:"port"`
	AuthUser      string   `json:"auth_user"`
	AuthPass      string   `json:"auth_pass"`
	FromAddr      string   `json:"from_addr"`
	ToAddrErrors  []string `json:"to_addr_errors"`
	ToAddrReports []string `json:"to_addr_reports"`
}

// read json mailing data
func readMailingData(dataFile string) (MailData, error) {
	var result MailData

	// open file to read
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return result, fmt.Errorf("failed to read mailing data file:\n\t%v", err)
	}

	// read file content
	errU := json.Unmarshal(data, &result)
	if errU != nil {
		return result, fmt.Errorf("failed to unmarshall mailing data:\n\t%v", errU)
	}

	return result, nil
}

// Send plain text mail without auth(typically smtp:25).
// msgType may be: 'report' or 'error'. Correspondingly will send either to ToAddrErrors or ToAddrReports recievers list in MailData
// appName - your app name. Will be represented in Subject: 'appName - msgType(02.01.2006 15:04)'
// subject will be like "appName - msgType"
func SendPlainEmailWoAuth(mailDataFile, msgType, appName string, msg []byte) error {
	// setting date string for Subject
	curDate := time.Now().Format("02.01.2006 15:04")
	// read mailing data
	mailData, err := readMailingData(mailDataFile)
	if err != nil {
		return fmt.Errorf("failed to get mailing data file:\n\t%v", err)
	}

	// setting mail params
	fromAddr := mailData.FromAddr
	smtpHost := mailData.Host
	smtpHostAndPort := fmt.Sprintf("%s:%s", smtpHost, mailData.Port)
	subject := fmt.Sprintf("%s - %s(%v)\n", appName, msgType, curDate)

	// checking type of recepients to implement(errors/reports)
	var toAddr []string
	switch msgType {
	case "error":
		toAddr = mailData.ToAddrErrors
	case "report":
		toAddr = mailData.ToAddrReports
	default:
		return fmt.Errorf("wrong msgType: neither 'error' nor 'report'")
	}
	// set "TO:"" header - must be comma separated values string
	toHeader := strings.Join(toAddr, ",")

	// setting message body
	// Generate a random Message-ID
	// r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// messageID := strconv.FormatInt(r.Int63(), 10) + "@" + smtpHost
	message := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s>\n\n%v", fromAddr, toHeader, subject, string(msg))

	// Send the email
	errS := smtp.SendMail(smtpHostAndPort, nil, fromAddr, toAddr, []byte(message))
	if errS != nil {
		return fmt.Errorf("error in SendMail func:\n\t%v", errS)
	}

	return nil
}
