package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
)

// Config represent the config.json file. When adding a key in json, we must add it here also to be able to fetch it
type Config struct {
	PreviewUrlTemplate  string            `json:"preview_url_template"`
	LinearOrganization  string            `json:"linear_organization"`
	LinearTicketPrefix  string            `json:"linear_ticket_prefix"`
	DailyDirectory      string            `json:"daily_directory"`
	DailyFile           string            `json:"daily_file"`
	PipelineAliases     map[string]string `json:"pipeline_aliases"`
	PipelineSuffixes    map[string]string `json:"pipeline_suffixes"`
	PipelineUrlTemplate string            `json:"pipeline_url_template"`
}

func GetConfig(key string) string {
	value := getValue(key)

	return value.String()
}

func GetMapConfig(key string) map[string]string {
	value := getValue(key)

	return value.Interface().(map[string]string)
}

func getValue(key string) reflect.Value {
	jsonFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var config Config
	json.Unmarshal([]byte(byteValue), &config)

	r := reflect.ValueOf(&config).Elem()
	rt := r.Type()

	field, _ := rt.FieldByName(key)
	rv := reflect.ValueOf(&config)

	return reflect.Indirect(rv).FieldByName(field.Name)
}
