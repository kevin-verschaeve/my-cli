package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func GetAzureToken(data map[string]string) (*TokenResponse, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	form := url.Values{}
	for key, value := range data {
		form.Set(key, value)
	}

	azureTenant := GetConfig("AzureTenant")

	fmt.Println(azureTenant)

	resp, err := client.Post(
		"https://login.microsoftonline.com/"+azureTenant+"/oauth2/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Vérifier le code HTTP
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error: %s", body)
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
