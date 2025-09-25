package handler

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"reconconverter/config"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/ssh"
	"gopkg.in/gomail.v2"
)

type Handler struct {
	Config     *config.Config
	Client     *ssh.Client
	MailSender Sender
}

type Sender interface {
	DialAndSend(...*gomail.Message) error
}

func NewHandler(config *config.Config) *Handler {

	dialer := gomail.NewDialer(config.Smtp.Host, config.Smtp.Port, config.Smtp.User, config.Smtp.Password)
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	dialer.SSL = false

	return &Handler{
		Config:     config,
		MailSender: dialer,
	}
}

func (handler *Handler) OvoHandler() {
	conn, client, err := handler.CreateClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}

	defer client.Close()
	defer conn.Close()

}

func (handler *Handler) IndodanaHandler() {
	conn, client, err := handler.CreateClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
	}

	defer client.Close()
	defer conn.Close()

	files, err := client.ReadDir(handler.Config.Indodana.SourcePath)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
		return
	}

	for _, file := range files {
		var err error
		if file.IsDir() {
			continue // skip subdirectories
		}

		remoteFile, err := client.Open(handler.Config.Indodana.SourcePath + "/" + file.Name())
		if err != nil {
			log.Printf("Failed to open file %s: %v", file.Name(), err)
			continue
		}

		defer remoteFile.Close()

		localFile, err := os.Create(handler.Config.TempFolder + "/indodana/" + file.Name())
		if err != nil {
			log.Printf("Failed to create local file %s: %v", file.Name(), err)
			remoteFile.Close()
			continue
		}

		defer localFile.Close()

		_, err = io.Copy(localFile, remoteFile)
		if err != nil {
			log.Printf("Failed to copy file %s: %v", file.Name(), err)
		} else {
			log.Printf("Downloaded:", file.Name())
		}

		f, err := excelize.OpenFile(localFile.Name())
		if err != nil {
			log.Fatalf("Got error %v", err)
			return
		}

		// var header []string
		var content [][]string

		rows, err := f.GetRows("Ledger")
		if err != nil {
			log.Fatalf("Got error when get rows %v", err)
		}

		var countBefore int
		for idx, each := range rows {
			if idx > 0 {
				countBefore++
			}
			content = append(content, each)
		}

		outputDir := "./converted"

		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			log.Fatalf("Error when create directory %v", err)
			return
		}

		newFilename := strings.ReplaceAll(localFile.Name(), "_yokke-ptp", "")
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

		fmt.Println("INDODANA file " + localFile.Name() + " converted to ---->  " + newFilename + " successfully")

		dstFile, err := client.Create(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			log.Fatalf("Failed to put file %v to sftp server. Err: ", newFilename, err.Error())
		}

		defer dstFile.Close()

		_, err = io.Copy(dstFile, file)
		if err != nil {
			log.Fatalf("Failed to put file %v to sftp server. Err:", file.Name(), err.Error())
		}

		log.Printf("Success converting file")

		// read again to count row after converted
		convertedFile, err := client.Open(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			log.Fatalf("Failed to read file (converted) %s: %v", newFilename, err)
		}

		defer convertedFile.Close()

		f, err = excelize.OpenReader(convertedFile)
		if err != nil {
			log.Fatalf("Got error %v", err)
			return
		}

		rowsAfter, err := f.GetRows("Sheet1")
		if err != nil {
			log.Fatalf("Got error when get rows %v", err)
		}

		var countAfter int
		for idx, _ := range rowsAfter {
			if idx > 0 {
				countAfter++
			}
		}

		log.Printf("Count before: %d", countBefore)
		log.Printf("Count after: %d", countAfter)

		return

	}

	// err := filepath.WalkDir(handler.Config.Indodana.SourcePath, func(path string, d os.DirEntry, err error) error {
	// 	if err != nil {
	// 		return err
	// 	}

	// 	if d.IsDir() || !strings.HasSuffix(d.Name(), ".xlsx") {
	// 		return nil
	// 	}

	// 	logrus.Infof("Reading file.... %v", d.Name())
	// 	if strings.HasPrefix(d.Name(), "Indodana") {
	// 		err = indodanaHandler(path, d.Name())
	// 		if err != nil {
	// 			return err
	// 		}
	// 	} else {
	// 		err = ovoHandler(path, d.Name())
	// 	}

	// 	return nil
	// })
	// var err error

	// f, err := excelize.OpenFile(handler.Config.Indodana.SourcePath)
	// if err != nil {
	// 	log.Fatalf("Got error %v", err)
	// 	return
	// }

	// // var header []string
	// var content [][]string

	// rows, err := f.GetRows("Ledger")
	// if err != nil {
	// 	log.Fatalf("Got error when get rows %v", err)
	// }
	// for _, each := range rows {
	// 	content = append(content, each)
	// }

	// outputDir := "./converted"

	// err = os.MkdirAll(outputDir, 0755)
	// if err != nil {
	// 	log.Fatalf("Error when create directory %v", err)
	// 	return err
	// }

	// newFilename := strings.ReplaceAll(originalFilename, "_yokke-ptp", "")
	// newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

	// newFilename = newFilename + ".csv"
	// file, err := os.Create(outputDir + "/" + newFilename)

	// writer := csv.NewWriter(file)
	// writer.Comma = ';'

	// for _, each := range content {
	// 	if err := writer.Write(each); err != nil {
	// 		panic(err)
	// 	}
	// }

	// writer.Flush()

	// fmt.Println("INDODANA file " + originalFilename + " converted to ---->  " + newFilename + " successfully")

	// return err
}

func (handler *Handler) CreateClient() (conn *ssh.Client, cl *sftp.Client, e error) {
	var err error
	sshConfig := &ssh.ClientConfig{
		User: handler.Config.Sftp.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(handler.Config.Sftp.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err = ssh.Dial("tcp", handler.Config.Sftp.Host+":"+strconv.Itoa(handler.Config.Sftp.Port), sshConfig)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
		return nil, nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Fatalf("Failed to create SFTP client: %v", err)
		return conn, nil, err
	}

	return conn, client, err
}
