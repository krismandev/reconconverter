package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"reconconverter/config"
	"reconconverter/handler"
	"reconconverter/mail"
	"reconconverter/utils"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/xuri/excelize/v2"
)

var app = cli.NewApp()

type Application struct {
	Config *config.Config
}

func (*Application) JobHandler() {

}

func main() {

	c := cron.New()

	logFile, err := os.OpenFile("miniprogram"+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logrus.Fatalf("Failed to create logfile %v", err)
	}

	writers := []io.Writer{
		os.Stdout,
		logFile,
	}
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&utils.CustomJSONFormatter{})
	logrus.SetOutput(io.MultiWriter(writers...))

	config := &config.Config{}
	configFile := "./config.yaml"

	config.LoadYAML(&configFile)

	assets, err := mail.NewAssets("./views", mail.NotifConverted)
	if err != nil {
		panic(err)
	}

	handler := handler.NewHandler(config, assets)

	cronList := strings.Split(config.Cron, ",")

	for _, each := range cronList {
		_, err := c.AddFunc(each, func() {
			counter := 0
			for counter < 4 {
				counter++
				handler.IndodanaHandler()
				handler.OvoHandler()
				// duration := time.Duration()
				time.Sleep(time.Duration(config.JobLoopDelay) * time.Minute)
			}
		})

		if err != nil {
			logrus.Fatalf("Error initiate cron : %v", err)
		}
	}

	// c.AddFunc("* * * * *", func() {
	// 	handler.BackupCleanerIndodana()
	// 	handler.BackupCleanerOvo()
	// })

	c.Start()

	select {}

}

func initCommands() {
	app.Commands = []cli.Command{
		{
			Name:        "ovo",
			Description: "Convert OVO File",
			Action: func(c *cli.Context) {

			},
		},
		{
			Name:        "indodana",
			Description: "Convert OVO File",
			Action: func(c *cli.Context) {

			},
		},
	}
}

func ovoHandler(path, originalFilename string) error {
	var err error

	f, err := excelize.OpenFile(path)
	if err != nil {
		logrus.Errorf("Got error %v", err)

		return err
	}

	sheetList := f.GetSheetList()

	sheet := sheetList[0]
	var content [][]string

	rows, err := f.GetRows(sheet)
	for _, each := range rows {
		content = append(content, each)
	}

	outputDir := "./converted"

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		logrus.Errorf("Error when create directory %v", err)
		return err
	}

	re := regexp.MustCompile(`(\d{2})-(\d{2})-(\d{4})`)

	// Replace with format YYYY/MM/DD
	newFilename := re.ReplaceAllString(originalFilename, "$3$2$1")
	newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

	newFilename = newFilename + ".csv"
	file, err := os.Create(outputDir + "/" + newFilename)

	writer := csv.NewWriter(file)
	writer.Comma = ';'

	for idx, each := range content {
		if idx < len(content) {
			if err := writer.Write(each); err != nil {
				panic(err)
			}
		}
	}

	writer.Flush()

	fmt.Println("OVO file  " + originalFilename + " converted to ---->  " + newFilename + " successfully")

	return err
}

func indodanaHandler(path string, originalFilename string) error {
	var err error

	f, err := excelize.OpenFile(path)
	if err != nil {
		logrus.Errorf("Got error %v", err)

		return err
	}

	// var header []string
	var content [][]string

	rows, err := f.GetRows("Ledger")
	if err != nil {
		logrus.Errorf("Got error when get rows %v", err)
	}
	for _, each := range rows {
		content = append(content, each)
	}

	outputDir := "./converted"

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		logrus.Errorf("Error when create directory %v", err)
		return err
	}

	newFilename := strings.ReplaceAll(originalFilename, "_yokke-ptp", "")
	newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

	newFilename = newFilename + ".csv"
	file, err := os.Create(outputDir + "/" + newFilename)

	writer := csv.NewWriter(file)
	writer.Comma = ';'

	for _, each := range content {
		if err := writer.Write(each); err != nil {
			panic(err)
		}
	}

	writer.Flush()

	fmt.Println("INDODANA file " + originalFilename + " converted to ---->  " + newFilename + " successfully")

	return err
}
