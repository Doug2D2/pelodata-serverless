package shared

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

const basePelotonURL = "https://api.onepeloton.com"

// PelotonRequest calls the Peloton API
func PelotonRequest(method, url string, headers map[string]string, body io.Reader) ([]byte, http.Header, int, error) {
	if !strings.HasPrefix(url, "/") {
		url = fmt.Sprintf("/%s", url)
	}

	fullURL := fmt.Sprintf("%s%s", basePelotonURL, url)

	client := &http.Client{}
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, fmt.Errorf("Unable to generate http request: %s", err.Error())
	}

	// Add peloton required header
	req.Header.Add("Peloton-Platform", "web")
	for key, val := range headers {
		req.Header.Add(key, val)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, fmt.Errorf("Unable to get categories from Peloton: %s", err.Error())
	}
	defer resp.Body.Close()

	resBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, fmt.Errorf("Unable to read response body: %s", err.Error())
	}

	if resp.StatusCode > 399 {
		return resBody, resp.Header, resp.StatusCode, fmt.Errorf("Error communicating with Peloton: %s", resp.Status)
	}

	return resBody, resp.Header, http.StatusOK, nil
}
