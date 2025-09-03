package app

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
)

const CONFIG_FILE string = "config.json"

type ApplicationConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
}

// Config represent the config.json file. When adding a key in json, we must add it here also to be able to fetch it
type Config struct {
	VersionControlService string                       `json:"vcs"`
	PreviewUrlTemplate    string                       `json:"preview_url_template"`
	LinearOrganization    string                       `json:"linear_organization"`
	LinearTicketPrefix    string                       `json:"linear_ticket_prefix"`
	DailyFile             string                       `json:"daily_file"`
	AzureTenant           string                       `json:"azure_tenant"`
	Applications          map[string]ApplicationConfig `json:"applications"`
}

func GetConfig(key string) string {
	value := getValue(key)

	return value.String()
}

func GetMapConfig(key string) map[string]string {
	value := getValue(key)

	return value.Interface().(map[string]string)
}

func (c *Config) GetApplication(name string) (*ApplicationConfig, bool) {
	app, ok := c.Applications[name]
	if !ok {
		return nil, false
	}
	return &app, true
}

func LoadConfig() (*Config, error) {
	configPath := MyCliHome() + "/" + CONFIG_FILE
	jsonFile, err := os.Open(configPath)

	if err != nil {
		return nil, err
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var config Config
	if err := json.Unmarshal([]byte(byteValue), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getValue(key string) reflect.Value {
	config, _ := LoadConfig()

	r := reflect.ValueOf(config).Elem()
	rt := r.Type()

	field, _ := rt.FieldByName(key)
	rv := reflect.ValueOf(config)

	return reflect.Indirect(rv).FieldByName(field.Name)
}
