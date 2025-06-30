package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/xuri/excelize/v2"
)

var app = cli.NewApp()

func main() {

	folder := "."
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".xlsx") {
			return nil
		}

		logrus.Infof("Reading file.... %v", d.Name())
		if strings.HasPrefix(d.Name(), "Indodana") {
			err = indodanaHandler(path, d.Name())
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		logrus.Errorf("Got error : %v", err)
	} else {
		logrus.Info("Success")
	}

	fmt.Println("Press ENTER to exit")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	// time.Sleep(2 * time.Second)

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

	fmt.Println("file " + originalFilename + " converted to ---->  " + newFilename + " successfully")

	return err
}
