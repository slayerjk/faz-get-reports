package hdnaumenapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// response struct for json body of getData
type getDataResponse struct {
	Fields struct {
		Message struct {
			Header struct {
				ServiceCall struct {
					UUID  string `json:"UUID"`
					Title string `json:"title"`
				} `json:"serviceCall"`
				Condition string `json:"condition"`
			} `json:"header"`
		} `json:"message"`
	} `json:"fields"`
}

// response struct for json body of getData
type getTaskDetailsResponse struct {
	SumDescription string `json:"sumDescription"`
}

// http(api) client(insecure)
func NewApiInsecureClient() http.Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		MaxIdleConns:    10,
		IdleConnTimeout: 30 & time.Second,
	}

	client := &http.Client{Transport: transport}

	return *client
}

// Get ServiceCall and task id(RP) based on data parameter
//
// Example of full URL to get ServiceCall:
//
// https://{{base_url}}/gateway/services/rest/getData?accessKey={{accessKey}}&params={{example: data$123456}},user
//
// return []string : serviceCall, RP
func GetServiceCallAndRP(c *http.Client, baseUrl, accessKey, taskId string) ([]string, error) {
	var respData getDataResponse
	result := make([]string, 0, 2)

	// form request URL
	requestURL := fmt.Sprintf("%s/gateway/services/rest/getData?accessKey=%s&params=%s,user", baseUrl, accessKey, taskId)
	// fmt.Println(request)

	// form GET request
	request, errReq := http.NewRequest(http.MethodGet, requestURL, nil)
	if errReq != nil {
		return nil, fmt.Errorf("failed to form request of getData:\n\t%v", errReq)
	}

	// make request
	response, errResp := c.Do(request)
	if errResp != nil {
		return nil, fmt.Errorf("failed to make request of getData:\n\t%v", errResp)
	}

	// response status must be 200
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("bad response status code of getData: %v", response.Status)
	}

	// read response
	respBody, errR := io.ReadAll(response.Body)
	if errR != nil {
		return nil, fmt.Errorf("failed to read response of getData:\n\t%v", errR)
	}

	// unmarshalling json body int var
	errU := json.Unmarshal(respBody, &respData)
	if errU != nil {
		return nil, fmt.Errorf("failed to unmarshall response of getData:\n\t%v\n\t%s", errU, string(respBody))
	}

	// adding ServiceCall & RP to result
	result = append(result, respData.Fields.Message.Header.ServiceCall.UUID, respData.Fields.Message.Header.ServiceCall.Title)

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
	serviceCallAndRR, errG := GetServiceCallAndRP(c, baseUrl, accessKey, taskId)
	if errG != nil {
		return nil, fmt.Errorf("failed to get ServiceCall and RP:\n\t%v", errG)
	}

	// form request URL
	requestURL := fmt.Sprintf("%s/sd/services/rest/get/%s?accessKey=%s", baseUrl, serviceCallAndRR[0], accessKey)
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

	// response status must be 200
	if response.StatusCode != 200 {
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
	result = append(result, serviceCallAndRR[0], serviceCallAndRR[1], respData.SumDescription)

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

	// checking response status code: must be 200
	if response.StatusCode != 200 && response.StatusCode != 202 {
		return fmt.Errorf("check TakeSCResponsibility status: %v", response.Status)
	}

	return nil
}
