package hdnaumenapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// response struct for json body of getData
type getDataResponse struct {
	Fields struct {
		SC string `json:"uuidInMainSyst"`
	} `json:"fields"`
}

// response struct for json body of getData
type getTaskDetailsResponse struct {
	SumDescription string `json:"sumDescription"`
	RP             string `json:"title"`
}

// Get ServiceCall and task id(RP) based on data parameter
//
// Example of full URL to get ServiceCall:
//
// https://{{base_url}}/gateway/services/rest/getData?accessKey={{accessKey}}&params={{example: data$123456}},user
//
// return []string : serviceCall, RP
// func GetServiceCallAndRP(c *http.Client, baseUrl, accessKey, taskId string) ([]string, error) {
func GetServiceCall(c *http.Client, baseUrl, accessKey, taskId string) (string, error) {
	var respData getDataResponse
	// result := make([]string, 0, 2)
	var result string

	// form request URL
	requestURL := fmt.Sprintf("%s/gateway/services/rest/getData?accessKey=%s&params=%s,user", baseUrl, accessKey, taskId)
	// fmt.Println(request)

	// form GET request
	request, errReq := http.NewRequest(http.MethodGet, requestURL, nil)
	if errReq != nil {
		return "", fmt.Errorf("failed to form request of getData:\n\t%v", errReq)
	}

	// make request
	response, errResp := c.Do(request)
	if errResp != nil {
		return "", fmt.Errorf("failed to make request of getData:\n\t%v", errResp)
	}
	defer response.Body.Close()

	// response status must be 200
	if response.StatusCode != 200 && response.StatusCode != 202 {
		return "", fmt.Errorf("bad response status code of getData: %v", response.Status)
	}

	// read response
	respBody, errR := io.ReadAll(response.Body)
	if errR != nil {
		return "", fmt.Errorf("failed to read response of getData:\n\t%v", errR)
	}

	// unmarshalling json body int var
	errU := json.Unmarshal(respBody, &respData)
	if errU != nil {
		return "", fmt.Errorf("failed to unmarshall response of getData:\n\t%v\n\t%s", errU, string(respBody))
	}

	// adding ServiceCall & RP to result
	result = respData.Fields.SC
	if len(result) == 0 {
		return "", fmt.Errorf("failed to find any ServiceCall, empty result")
	}

	return result, nil
}

// Get Request details based on serviceCall
//
// Example of full URL to get details:
//
// https://{{base_url}}/sd/services/rest/get/{{example: serviceCall$1234567}}?accessKey={{accessKey}}
//
// returns []string: ServiceCall from GetServiceCallAndRP, RP from GetServiceCallAndRP, sumDescription json key's value
func GetTaskSumDescriptionAndRP(c *http.Client, baseUrl, accessKey, taskId string) ([]string, error) {
	var respData getTaskDetailsResponse
	result := make([]string, 0, 2)

	// get ServiceCall id & RP id
	serviceCall, errG := GetServiceCall(c, baseUrl, accessKey, taskId)
	if errG != nil {
		return nil, fmt.Errorf("failed to get ServiceCall:\n\t%v", errG)
	}

	// form request URL
	requestURL := fmt.Sprintf("%s/sd/services/rest/get/%s?accessKey=%s", baseUrl, serviceCall, accessKey)
	// fmt.Println(requestURL)

	// form GET request
	request, errReq := http.NewRequest(http.MethodGet, requestURL, nil)
	if errReq != nil {
		return nil, fmt.Errorf("failed to form request to get task details:\n\t%v", errReq)
	}

	// make GET request
	response, errR := c.Do(request)
	if errR != nil {
		return nil, fmt.Errorf("failed to make request to get task details:\n\t%v", errR)
	}
	defer response.Body.Close()

	// response status must be 200
	if response.StatusCode != 200 && response.StatusCode != 202 {
		return nil, fmt.Errorf("bad response status code of get task details: %v", response.Status)
	}

	// read response body
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response of get task details:\n\t%v", err)
	}

	// unmarshalling response body
	errU := json.Unmarshal(respBody, &respData)
	if errU != nil {
		return nil, fmt.Errorf("failed to unmarshall response of get task details:\n\t%v\n\t%s", errU, string(respBody))
	}
	// fmt.Printf("%+v", respData)

	// append to result
	result = append(result, serviceCall, respData.RP, respData.SumDescription)

	return result, nil
}

// Take responsibility on Naumen ticket(GET)
//
// Example of request:
//
// https://{{base_url}}/gateway/services/rest/takeSCResponsibility?accessKey={{accessKey}}&params='{{serviceCall}}',user
func TakeSCResponsibility(c *http.Client, baseUrl, accessKey, serviceCall string) error {
	// form request URL
	requestURL := fmt.Sprintf("%s/gateway/services/rest/takeSCResponsibility?accessKey=%s&params='%s',user", baseUrl, accessKey, serviceCall)

	// form GET request
	request, errReq := http.NewRequest(http.MethodGet, requestURL, nil)
	if errReq != nil {
		return fmt.Errorf("failed to form TakeSCResponsibility request:\n\t%v", errReq)
	}

	// making request
	response, errResp := c.Do(request)
	if errResp != nil {
		return fmt.Errorf("failed to make TakeSCResponsibility request:\n\t%v\n\t%s", errResp, requestURL)
	}
	defer response.Body.Close()

	// checking response status code: must be 200
	if response.StatusCode != 200 && response.StatusCode != 202 {
		return fmt.Errorf("check TakeSCResponsibility status: %v", response.Status)
	}

	return nil
}

// Attach list of files to Naumen task and set 'ready for acceptance'
//
// Request URL example:
//
// https://{{base_url}}/gateway/services/rest/waitingForAccept?accessKey={{accessKey}}&params='{{serviceCall}}',request,user
//
// body form-data example:
//
// procCodeclose(TEXT; Resolved) = catalogs$28411
// solution(TEXT) = text
// files(FILE) = files
func AttachFilesAndSetAcceptance(c *http.Client, baseURL, accessKey, serviceCall string, files []string) error {
	// form request URL
	requestURL := fmt.Sprintf("%s/gateway/services/rest/waitingForAccept?accessKey=%s&params='%s',request,user", baseURL, accessKey, serviceCall)

	// forming body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// dataForm := url.Values{
	// 	"procCodeClose": {"catalogs$28411"},
	// 	"solution":      {"Запрос  исполнен, результат во вложении!"},
	// 	"files":         {strings.Join(files, ",")},
	// }
	// requestBody := strings.NewReader(dataForm.Encode())

	// write files to body
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open path(%s):\n\t%v", path, err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("files", filepath.Base(path))
		if err != nil {
			return fmt.Errorf("failed to CreateFormFile path(%s):\n\t%v", path, err)
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return fmt.Errorf("failed to io.Copy filepath data to form-data:\n\t%v", err)
		}
	}

	// write fields to body
	writer.WriteField("procCodeClose", "catalogs$28411")
	writer.WriteField("solution", "Запрос  исполнен, результат во вложении!")

	// close form data
	err := writer.Close()
	if err != nil {
		return err
	}

	// forming request
	request, errReq := http.NewRequest(http.MethodPost, requestURL, &body)
	if errReq != nil {
		return fmt.Errorf("failed to form AttachFilesAndSetAcceptance request:\n\t%v", errReq)
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	// making request
	response, errResp := c.Do(request)
	if errResp != nil {
		return fmt.Errorf("failed to make AttachFilesAndSetAcceptance request:\n\t%v", errResp)
	}
	defer response.Body.Close()

	// checking status
	if response.StatusCode != 200 && response.StatusCode != 202 {
		return fmt.Errorf("bad response status code of AttachFilesAndSetAcceptance: %v", response.Status)
	}

	return nil
}
