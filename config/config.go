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
		SftpSource      Sftp   `yaml:"sftpSource"`
		SftpDestination Sftp   `yaml:"sftpDestination"`
		BackupPath      string `yaml:"backupPath"`
	} `yaml:"ovo"`
	Indodana struct {
		Interval        int    `yaml:"interval"`
		SourcePath      string `yaml:"sourcePath"`
		DestinationPath string `yaml:"destinationPath"`
		SftpSource      Sftp   `yaml:"sftpSource"`
		SftpDestination Sftp   `yaml:"sftpDestination"`
		BackupPath      string `yaml:"backupPath"`
	} `yaml:"indodana"`
	Sftp struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"sftp"`
	TempFolder string `yaml:"tempFolder"`
	Smtp       struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		From     string `yaml:"from"`
		To       string `yaml:"to"`
	} `yaml:"smtp"`
}

type Sftp struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
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
