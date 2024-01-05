package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
)

const CONFIG_FILE string = "config.json"

// Config represent the config.json file. When adding a key in json, we must add it here also to be able to fetch it
type Config struct {
	VersionControlService string            `json:"vcs"`
	PreviewUrlTemplate    string            `json:"preview_url_template"`
	LinearOrganization    string            `json:"linear_organization"`
	LinearTicketPrefix    string            `json:"linear_ticket_prefix"`
	DailyFile             string            `json:"daily_file"`
	PipelineAliases       map[string]string `json:"pipeline_aliases"`
	PipelineSuffixes      map[string]string `json:"pipeline_suffixes"`
	PipelineUrlTemplate   string            `json:"pipeline_url_template"`
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
	configPath := MyCliHome() + "/" + CONFIG_FILE
	jsonFile, err := os.Open(configPath)
	if err != nil {
		fmt.Println(err, fmt.Sprintf("Did you create the %s file ?", configPath))
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
