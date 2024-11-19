package fazrequests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"time"
)

const (
	errJsonMarshall   = "failed to marshal request body:\n\t%v"
	errJsonUnmarshall = "failed to unmarshal response body:\n\t%v"
	errRequest        = "failed to do request:\n\t%v"
	errReadResp       = "failed to read response:\n\t%v"
	errStatusCode     = "status code is not 200:\n\t%v\n\t%v"
	errMsgNotOk       = "response message is not ok:\n\t%v"
	errEmptyResult    = "result is empty"

	contentType = "application/json"
	messageOk   = `"message": "OK"`
)

// GET SESSION ID TO PERFORM FAZ API REQUESTS
func GetSessionid(httpClient *http.Client, fazurl, apiuser, apipass string) (string, error) {
	/*
		Correct Request Example:

			{
				"method": "exec",
				"params": [
					{
						"data": {
							"passwd": "{{pass}}",
							"user": "{{user}}"
						},
						"url": "/sys/login/user"
					}
				],
				"session": "1",
				"id": "1"
			}
	*/

	/*
		Correct Response Example:

			{
				"result": [
					{
						"status": {
							"code": 0,
							"message": "OK"
						},
						"url": "/sys/login/user"
					}
				],
				"session": "{{session}}",
				"id": "1"
			}
	*/

	/*
		Bad Response Example:

		{
			"result": [
				{
					"status": {
						"code": -22,
						"message": "Login fail"
					},
					"url": "/sys/login/user"
				}
			],
			"id": "1"
		}
	*/

	// FORMING STRUCTS FOR REQUEST & RESPONSE JSON
	type Request struct {
		Method  string `json:"method"`
		Session string `json:"session"`
		ID      string `json:"id"`
		Params  []struct {
			Data struct {
				Passwd string `json:"passwd"`
				User   string `json:"user"`
			} `json:"data"`
			URL string `json:"url"`
		} `json:"params"`
	}

	type Response struct {
		Result []struct {
			Status struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"status"`
			URL string `json:"url"`
		} `json:"result"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	// CREATING BODY STRUCT & RESPONSE VAR
	var sessionResp Response

	body := Request{
		Method:  "exec",
		Session: "1",
		ID:      "1",
		Params: []struct {
			Data struct {
				Passwd string `json:"passwd"`
				User   string `json:"user"`
			} `json:"data"`
			URL string `json:"url"`
		}{
			{
				Data: struct {
					Passwd string `json:"passwd"`
					User   string `json:"user"`
				}{
					Passwd: apipass,
					User:   apiuser,
				},
				URL: "/sys/login/user",
			},
		},
	}

	// FORMING JSON FOR REQUEST
	postBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf(errJsonMarshall, err)
	}
	requestBody := bytes.NewReader(postBody)

	// MAKING REQUEST
	resp, err := httpClient.Post(fazurl, contentType, requestBody)
	if err != nil {
		return "", fmt.Errorf(errRequest, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(errReadResp, err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
	}

	err = json.Unmarshal(respBody, &sessionResp)
	if err != nil {
		return "", fmt.Errorf(errJsonUnmarshall, err)
	}

	if len(sessionResp.Session) == 0 {
		return "", fmt.Errorf(errEmptyResult)
	}

	return sessionResp.Session, nil
}

// GET FAZ REPORT LAYOUT BY IT'S NAME
func GetFazReportLayout(httpClient *http.Client, fazurl, sessionid, adom, repName string) (int, error) {
	/*
		Correct Request Example

		{
			"method": "get",
			"params": [
				{
					"url": "report/adom/{{adom}}/config/layout",
					"apiver": 3,
					// "filter": [],
					// "sortings": []
					"data": {

					}
				}
			],
			"jsonrpc": "2.0",
			"session": "{{sessionid}}",
			"id": "10"
		}
	*/

	/*
				Correct Response Example(trimmed)

				{
		    "jsonrpc": "2.0",
		    "result": {
		        "status": {
		            "code": 0,
		            "message": "OK"
		        },
		        "data": [
		            {
		                "alignment": 0,
		                "bg-color": "#FFFFFF",
		                "body": "<h1>Bandwidth and A...",
		                "category": "Security",
		                "chart-heading-level": 2,
		                "chart-info-display": 0,
		                "component": null,
		                "coverpage-background-image": "{sys_img_path}/def_cover_bgimg_ver1.png",
		                "coverpage-enable-create-time": 1,
		                "coverpage-enable-time-period": 1,
		                "coverpage-footer-bgcolor": "transparent",
		                "coverpage-text-color": "#000000",
		                "coverpage-title": "{default}",
		                "coverpage-top-image-position": 1,
		                "description": "Security Analysis of traffic, ...",
		                "dev-type": 0,
		                "folders": [
		                    {
		                        "folder-id": 99999
		                    }
		                ],
		                "font-color": "#000000",
		                "font-family": "Open Sans",
		                "font-size": 12,
		                "font-type": 0,
		                "footer": [
		                    {
		                        "footer-id": 1,
		                        "graphic": null,
		                        "text": null,
		                        "type": 9
		                    }
		                ],
		                "footer-bgcolor": "#FFFFFF",
		                "header": [
		                    {
		                        "graphic": "fortinet_grey.png",
		                        "header-id": 1,
		                        "text": null,
		                        "type": 1
		                    }
		                ],
		                "header-bgcolor": "#FFFFFF",
		                "hide-report-title": 0,
		                "hide-rowid": 0,
		                "include-empty-charts": 1,
		                "is-template": 0,
		                "language": "en",
		                "layout-id": 1,
		                "left-margin": 6,
		                "protected": 0,
		                "right-margin": 6,
		                "title": "Security Analysis"
		            },
					{...}
	*/

	// FORMING REQUEST & RESPONSE STRUCTS
	type Request struct {
		Method string `json:"method"`
		Params []struct {
			URL    string `json:"url"`
			Apiver int    `json:"apiver"`
		} `json:"params"`
		Jsonrpc string `json:"jsonrpc"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	type Response struct {
		Jsonrpc string `json:"jsonrpc"`
		Result  struct {
			Status struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"status"`
			Data []struct {
				LayoutID int    `json:"layout-id"`
				Title    string `json:"title"`
			} `json:"data"`
		} `json:"result"`
		ID string `json:"id"`
	}

	// FORMING REQUEST & RESPONSE VARS
	request := Request{
		Method:  "get",
		Jsonrpc: "2.0",
		Session: sessionid,
		ID:      "2",
		Params: []struct {
			URL    string `json:"url"`
			Apiver int    `json:"apiver"`
		}{
			{
				URL:    fmt.Sprintf("report/adom/%s/config/layout", adom),
				Apiver: 3,
			},
		},
	}

	var response Response

	// REGEXP TO CHECK RESPONSE IS OK
	reMessageOK := regexp.MustCompile(messageOk)

	// FORMING REQUEST JSON
	reqBody, err := json.Marshal(request)
	if err != nil {
		return 0, fmt.Errorf(errJsonMarshall, err)
	}

	reqBodyBytges := bytes.NewReader(reqBody)

	// MAKING REQUEST & AND CHECKING IT'S CORRECT
	resp, err := httpClient.Post(fazurl, contentType, reqBodyBytges)
	if err != nil {
		return 0, fmt.Errorf(errRequest, err)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf(errReadResp, err)
	}

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
	}

	if !reMessageOK.Match(respBody) {
		return 0, fmt.Errorf(errMsgNotOk, string(respBody))
	}

	// UNMARSHALLING RESPONSE
	errRespJson := json.Unmarshal(respBody, &response)
	if errRespJson != nil {
		return 0, fmt.Errorf(errJsonUnmarshall, errRespJson)
	}

	// SEARCHING FOR CORRECT LAYOUT IN RESONSE
	for _, item := range response.Result.Data {
		if item.Title == repName {
			return item.LayoutID, nil
		}
	}

	return 0, fmt.Errorf(errEmptyResult)
}

// UPDATING DATASETS FOR REPORTS FOR CORRESPONDING USER
func UpdateDatasets(httpClient *http.Client, fazurl, sessionid, adom, username string, datasets []map[string]string) error {
	/*
		Correct Request Example(EVERY CONNECT):

		{
			"method": "update",
			"params": [
				{
					"url": "report/adom/{{adom}}/config/dataset/{{dataset}}",
					"apiver": 3,
					"data": {
						"query": "XXX"
					}
				}
			],
			"jsonrpc": "2.0",
			"session": "{{sessionid}}",
			"id": "3.1"
		}
	*/

	/*
		Correct Response Example:

		{
			"jsonrpc": "2.0",
			"result": {
				"status": {
					"code": 0,
					"message": "OK"
				},
				"data": {
					"name": "{{dataset}}"
				}
			},
			"id": "3.1"
		}
	*/

	/*
		Bad Response Example:

		{
			"jsonrpc": "2.0",
			"result": {
				"status": {
					"code": -3,
					"message": "Object does not exist"
				}
			},
			"id": "4.1"
		}
			OR

		{
			"result": [
				{
					"status": {
						"code": -11,
						"message": "No permission for the resource"
					}
				}
			],
			"id": "4.1"
		}
	*/

	// DATASETS URLS & QUERIES VARS
	var (
		datasetUrl   string
		datasetQuery string
	)
	// datasetUrlUpdAll := fmt.Sprintf("report/adom/%s/config/dataset/%s", adom, datasetAll)
	// datasetUrlUpdTotal := fmt.Sprintf("report/adom/%s/config/dataset/%s", adom, datasetTotal)

	// DATASETS INITIAL QUERIES FOR ALL & TOTAL
	// queryAll := "select from_itime(itime) as itime, `action`, \"user\", duration\nfrom $log where $filter and action in ('tunnel-up', 'tunnel-down') and tunneltype = 'ssl-web' and UPPER(`user`) LIKE UPPER('%USERNAME%')\norder by itime, \"user\", `action`, duration"
	// queryTotal := "select \"user\", $DAY_OF_MONTH as day, sum(duration) as duration, action, tunneltype\nfrom $log where $filter  and action = 'tunnel-down' and tunneltype = 'ssl-web' and UPPER(`user`) LIKE UPPER('%USERNAME%')\ngroup by day, duration, \"user\", action, tunneltype\norder by day"

	// REGEXP TO SUBSTITUTE USERNAME IN DATASET QUERY
	re := regexp.MustCompile(`%\w+%`)
	// REGEXP TO CHECK RESPONSE IS OK
	reMessageOK := regexp.MustCompile(messageOk)

	// FORMING DATASET STRUCT FOR REQUEST
	type Request struct {
		Method string `json:"method"`
		Params []struct {
			URL    string `json:"url"`
			Apiver int    `json:"apiver"`
			Data   struct {
				Query string `json:"query"`
			} `json:"data"`
		} `json:"params"`
		Jsonrpc string `json:"jsonrpc"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	datasetToUpd := Request{
		Method: "update",
		Params: []struct {
			URL    string `json:"url"`
			Apiver int    `json:"apiver"`
			Data   struct {
				Query string `json:"query"`
			} `json:"data"`
		}{
			{
				URL:    "XXX",
				Apiver: 3,
				Data: struct {
					Query string `json:"query"`
				}{
					Query: "XXX",
				},
			},
		},
		Jsonrpc: "2.0",
		Session: sessionid,
		ID:      "3",
	}

	// ITERATING THROUGH DATASETS & UPDATE THEM
	for _, item := range datasets {
		// FORIMING DATASET QUERY & URL FOR REQUEST
		datasetUrl = fmt.Sprintf("report/adom/%s/config/dataset/%s", adom, item["dataset"])
		datasetQuery = re.ReplaceAllLiteralString(item["dataset-query"], username)

		// FORMING JSON FOR DataAll
		datasetToUpd.Params[0].URL = datasetUrl
		datasetToUpd.Params[0].Data.Query = datasetQuery

		json, err := json.Marshal(datasetToUpd)
		if err != nil {
			return fmt.Errorf(errJsonMarshall, err)
		}
		requestBody := bytes.NewReader(json)

		// UPDATING DATASET
		resp, err := httpClient.Post(fazurl, contentType, requestBody)
		if err != nil {
			return fmt.Errorf(errRequest, err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf(errReadResp, err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
		}

		// CHECKING CORRECT RESPONSE JSON FOR DATASET ALL
		if !reMessageOK.MatchString(string(respBody)) {
			return fmt.Errorf(errMsgNotOk, string(respBody))
		}
	}

	return nil
}

// GETTING REPORT STATE(running/generated) TO CHECK IF IT READY TO DOWNLOAD
func reportIsGenerated(httpClient *http.Client, fazurl, sessionid, adom, repId string) (string, error) {
	/*
		Correct Request Example:

		{
			"jsonrpc": "2.0",
			"method": "get",
			"params": [
				{
					"apiver": 3,
					"url": "/report/adom/{{adom}}/run/{{repId}}"
				}
			],
			"session": "{{sessionid}}",
			"id": "8"
		}
	*/

	/*
		Correct Response Example:

		{
			"jsonrpc": "2.0",
			"result": {
				"device": {
					"count": 1,
					"data": "xxx"
				},
				"name": "xxx",
				"devtype": "FortiGate",
				"schedule_color": "",
				"title": "<REPORT NAME>",
				"tid": "{{repId}}",
				"date": "2024_08_06",
				"adminuser": "XXX",
				"profileid": "XXX",
				"start": "2024/08/06 11:03:29",
				"timestamp-start": 1722924209,
				"end": "2024/08/06 11:03:33",
				"timestamp-end": 1722924213,
				"period-start": "2024/08/04 00:00",
				"period-end": "2024/08/04 23:59",
				"state": "generated",
				"progress-percent": 100,
				"format": [
					"HTML",
					"PDF",
					"XML",
					"CSV",
					"JSON"
				]
			},
			"id": "8"
		}
	*/

	/*
		Bad Response Example:

		{
			"jsonrpc": "2.0",
			"error": {
				"code": -32603,
				"message": "Internal error: invalid uuid!"
			},
			"id": "8"
		}

		OR

		{
			"result": [
				{
					"status": {
						"code": -11,
						"message": "No permission for the resource"
					}
				}
			],
			"id": "8"
		}
	*/

	var result string

	// REGEXP TO CHECK REPORT STATE
	reGenerated := regexp.MustCompile(`"state": "generated"`)
	reRunning := regexp.MustCompile(`"state": "running"`)
	rePending := regexp.MustCompile(`"state": "pending"`)

	// FORMING STRUCT FOR REQUEST JSON
	type Request struct {
		Jsonrpc string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  []struct {
			Apiver int    `json:"apiver"`
			URL    string `json:"url"`
		} `json:"params"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	// FORMING REQUEST BODY
	body := Request{
		Jsonrpc: "2.0",
		Method:  "get",
		Session: sessionid,
		ID:      "4",
		Params: []struct {
			Apiver int    `json:"apiver"`
			URL    string `json:"url"`
		}{
			{
				Apiver: 3,
				URL:    fmt.Sprintf("/report/adom/%s/run/%s", adom, repId),
			},
		},
	}

	// FORMING REQUEST & MAKING REQUEST
	reqBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf(errJsonMarshall, err)
	}
	reqBodyBytes := bytes.NewReader(reqBody)

	resp, err := httpClient.Post(fazurl, contentType, reqBodyBytes)
	if err != nil {
		log.Fatal("FAILED: to Make Request(Report State):\n\t", err)
		return "", fmt.Errorf(errRequest, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(errReadResp, err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
	}

	switch {
	case reGenerated.MatchString(string(respBody)):
		// log.Printf("REPORT %s is ready\n", repId)
		result = "generated"
	case reRunning.MatchString(string(respBody)):
		// log.Printf("REPORT %s is still running", repId)
		result = "running"
	case rePending.MatchString(string(respBody)):
		// log.Printf("REPORT %s is Pending", repId)
		result = "pending"
	default:
		return "", fmt.Errorf("wrong response in reportIsGenerated result:\n\t%v", string(respBody))
	}

	return result, nil
}

// STARTING REPORTS PROCESSING
func StartReport(httpClient *http.Client, fazurl, adom, device, sessionid, start, end string, layout int) (string, error) {
	/*
		Correct Request Example:

		{
			"jsonrpc": "2.0",
			"method": "add",
			"params": [
				{
					"apiver": 3,
					"schedule-param": {
						"device": "{{device}}",
						"time-period": "other",
						"period-start": "00:00:01 2024/08/04",
						"period-end": "23:59:59 2024/08/04",
						"layout-id": 8
					},
					"url": "/report/adom/{{adom}}/run"
				}
			],
			"session": "{{sessionid}}",
			"id": "7"
		}
	*/

	/*
		Correct Response Example:

		{
			"jsonrpc": "2.0",
			"result": {
				"tid": "511d2c9c-5640-11ef-a0b0-7cc25579bc2e"
			},
			"id": "7"
		}
	*/

	// FORMING REQUEST STRUCT
	type Request struct {
		Jsonrpc string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  []struct {
			Apiver        int `json:"apiver"`
			ScheduleParam struct {
				Device      string `json:"device"`
				TimePeriod  string `json:"time-period"`
				PeriodStart string `json:"period-start"`
				PeriodEnd   string `json:"period-end"`
				LayoutID    int    `json:"layout-id"`
			} `json:"schedule-param"`
			URL string `json:"url"`
		} `json:"params"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	body := Request{
		Jsonrpc: "2.0",
		Method:  "add",
		Session: sessionid,
		ID:      "5",
		Params: []struct {
			Apiver        int `json:"apiver"`
			ScheduleParam struct {
				Device      string `json:"device"`
				TimePeriod  string `json:"time-period"`
				PeriodStart string `json:"period-start"`
				PeriodEnd   string `json:"period-end"`
				LayoutID    int    `json:"layout-id"`
			} `json:"schedule-param"`
			URL string `json:"url"`
		}{
			{
				Apiver: 3,
				URL:    fmt.Sprintf("/report/adom/%s/run", adom),
				ScheduleParam: struct {
					Device      string `json:"device"`
					TimePeriod  string `json:"time-period"`
					PeriodStart string `json:"period-start"`
					PeriodEnd   string `json:"period-end"`
					LayoutID    int    `json:"layout-id"`
				}{
					Device:      device,
					TimePeriod:  "other",
					PeriodStart: start,
					PeriodEnd:   end,
					LayoutID:    layout,
				},
			},
		},
	}

	// FORMING RESPONSE STRUCT
	type Response struct {
		Jsonrpc string `json:"jsonrpc"`
		Result  struct {
			Tid string `json:"tid"`
		} `json:"result"`
		ID string `json:"id"`
	}

	var response Response

	// MAKING REQUEST JSON
	reqBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf(errJsonMarshall, err)
	}

	reqBytes := bytes.NewBuffer(reqBody)

	// MAKING REQUEST
	resp, err := httpClient.Post(fazurl, contentType, reqBytes)
	if err != nil {
		return "", fmt.Errorf(errRequest, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(errReadResp, err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
	}

	// FORMING RESPONSE JSON
	errJson := json.Unmarshal(respBody, &response)
	if errJson != nil {
		return "", fmt.Errorf(errJsonUnmarshall, errJson)
	}

	if response.Result.Tid == "" {
		log.Fatalf("FAILED: to Get RepID:\n\t%s,\n\t%s", string(respBody), body.Params[0].URL)
		return "", fmt.Errorf(errEmptyResult)
	}

	// WAIT 5 SEC TO BYPASS PENDING
	time.Sleep(5 * time.Second)

	for {
		repState, err := reportIsGenerated(httpClient, fazurl, sessionid, adom, response.Result.Tid)
		if err != nil {
			return "", fmt.Errorf("error in reportIsGenerated:\n\t%v", err)
		}

		switch repState {
		case "pending":
			// log.Printf("Report is still Pending")
			time.Sleep(5 * time.Second)
			continue
		case "running":
			// log.Printf("Report is still Running")
			time.Sleep(10 * time.Second)
			continue
		case "generated":
			// log.Printf("Report is Ready")
			return response.Result.Tid, nil
		default:
			return "", fmt.Errorf("error in reportIsGenerated, wrong status:\n\t%v", repState)
		}
	}
}

// DOWNLOADING PDF REPORT
func DownloadPdfReport(httpClient *http.Client, fazUrl, fazAdom, sessionid, repId string) (string, error) {
	/*
		Correct Request Example:

		{
			"method": "get",
			"params": [
				{
					"url": "report/adom/{{adom}}/reports/data/99c2e40c-53b9-11ef-97a5-7cc25579bc2e",
					"apiver": 3,
					"format": "PDF",
					"data-type": "text"
				}
			],
			"jsonrpc": "2.0",
			"session": "{{sessionid}}",
			"id": "11"
		}
	*/

	/*
				Correct Response Example:

				{
		    "jsonrpc": "2.0",
		    "result": {
		        "name": "<REPORT NAME>",
		        "tid": "<REP ID>",
		        "data": "UEsDBBQAAAAIABNxDFlT0zsLbyECAGCJAgAYABw ...LONG LONG LINE"
		        "data-type": "zip/base64",
		        "checksum": {
		            "method": "MD5",
		            "hash": "<HASH>"
		        },
		        "length": 139829
		    },
		    "id": "11"
		}
	*/

	// FORMING REQUEST & RESPONSE STRUCT
	type Request struct {
		Method string `json:"method"`
		Params []struct {
			URL      string `json:"url"`
			Apiver   int    `json:"apiver"`
			Format   string `json:"format"`
			DataType string `json:"data-type"`
		} `json:"params"`
		Jsonrpc string `json:"jsonrpc"`
		Session string `json:"session"`
		ID      string `json:"id"`
	}

	type Response struct {
		Jsonrpc string `json:"jsonrpc"`
		Result  struct {
			Name     string `json:"name"`
			Tid      string `json:"tid"`
			Data     string `json:"data"`
			DataType string `json:"data-type"`
			Checksum struct {
				Method string `json:"method"`
				Hash   string `json:"hash"`
			} `json:"checksum"`
			Length int `json:"length"`
		} `json:"result"`
		ID string `json:"id"`
	}

	var (
		bodyResp Response
		result   string
	)

	bodyReq := Request{
		Method:  "get",
		Jsonrpc: "2.0",
		Session: sessionid,
		ID:      "6",
		Params: []struct {
			URL      string `json:"url"`
			Apiver   int    `json:"apiver"`
			Format   string `json:"format"`
			DataType string `json:"data-type"`
		}{
			{
				URL:      fmt.Sprintf("report/adom/%s/reports/data/%s", fazAdom, repId),
				Apiver:   3,
				Format:   "PDF",
				DataType: "text",
			},
		},
	}

	// MAKING REQUEST JSON
	jsonBody, err := json.Marshal(bodyReq)
	if err != nil {
		return "", fmt.Errorf(errJsonMarshall, err)
	}

	respBodyBytes := bytes.NewReader(jsonBody)

	// 	MAKING REQUEST
	resp, err := httpClient.Post(fazUrl, contentType, respBodyBytes)
	if err != nil {
		return "", fmt.Errorf(errRequest, err)
	}
	defer resp.Body.Close()

	// READING RESPONSE
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf(errReadResp, err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf(errStatusCode, resp.StatusCode, string(respBody))
	}

	// UNMARSHALLING RESP JSON
	errRespJson := json.Unmarshal(respBody, &bodyResp)
	if errRespJson != nil {
		return "", fmt.Errorf(errJsonUnmarshall, errRespJson)
	}

	// PROCESSING REPORT DATA
	result = bodyResp.Result.Data
	if result == "" {
		return "", fmt.Errorf(errEmptyResult)
	}

	return result, nil
}
