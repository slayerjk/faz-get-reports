package hdnaumenapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// Get ServiceCall and task id(RP) based on data parameter
//
// Example of full URL to get ServiceCall:
//
// https://{{base_url}}/gateway/services/rest/getData?accessKey={{accessKey}}&params={{example: data$123456}},user
//
// return []string : serviceCall, RP
func GetServiceCallAndRP(baseUrl, accessKey, taskId string) ([]string, error) {
	var respData getDataResponse
	result := make([]string, 0, 2)

	// form request
	request := fmt.Sprintf("%s/gateway/services/rest/getData?accessKey=%s&params=%s,user", baseUrl, accessKey, taskId)
	// fmt.Println(request)

	// make GET request
	response, errG := http.Get(request)
	if errG != nil {
		return nil, fmt.Errorf("failed to make request of getData:\n\t%v", errG)
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
// returns []string: RP from GetServiceCallAndRP && sumDescription json key's value
func GetTaskSumDescriptionAndRP(baseUrl, accessKey, taskId string) ([]string, error) {
	var respData getTaskDetailsResponse
	result := make([]string, 0, 2)

	// get ServiceCall id & RP id
	serviceCallAndRR, errG := GetServiceCallAndRP(baseUrl, accessKey, taskId)
	if errG != nil {
		return nil, fmt.Errorf("failed to get ServiceCall and RP:\n\t%v", errG)
	}

	// form request
	request := fmt.Sprintf("%s/sd/services/rest/get/%s?accessKey=%s", baseUrl, serviceCallAndRR[0], accessKey)
	fmt.Println(request)

	// make GET request
	response, errR := http.Get(request)
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
	result = append(result, serviceCallAndRR[1], respData.SumDescription)

	return result, nil
}
