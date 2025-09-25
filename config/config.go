package config

import (
	"io/ioutil"

	"github.com/go-yaml/yaml"
)

type Config struct {
	Ovo struct {
		Interval        int    `yaml:"interval"`
		SourcePath      string `yaml:"sourcePath"`
		DestinationPath string `yaml:"destinationPath"`
	} `yaml:"ovo"`
	Indodana struct {
		Interval        int    `yaml:"interval"`
		SourcePath      string `yaml:"sourcePath"`
		DestinationPath string `yaml:"destinationPath"`
	} `yaml:"ovo"`
	Sftp struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"sftp"`
	TempFolder string `yaml:"tempPath"`
	Smtp       struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"smtp"`
}

func (c *Config) LoadYAML(filename *string) error {
	raw, err := ioutil.ReadFile(*filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(raw, c)
	if err != nil {
		return err
	}
	return nil
}
