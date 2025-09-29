package handler

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"reconconverter/config"
	"reconconverter/mail"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/ssh"
	"gopkg.in/gomail.v2"
)

type Handler struct {
	Config     *config.Config
	Client     *ssh.Client
	MailSender mail.Sender
	Assets     *mail.Assets
}

func NewHandler(config *config.Config, assets *mail.Assets) *Handler {

	dialer := gomail.NewDialer(config.Smtp.Host, config.Smtp.Port, config.Smtp.User, config.Smtp.Password)
	dialer.Auth = smtp.PlainAuth("", config.Smtp.User, config.Smtp.Password, "mailer.yokke.co.id")
	dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	dialer.SSL = false

	if conn, err := dialer.Dial(); err != nil {
		logrus.Fatalf("failed to connect to smtp server: %v", err)
	} else if err := conn.Close(); err != nil {
		logrus.Fatalf("failed to close connection : %v", err)
	}

	logrus.Info("Connected to smtp")

	return &Handler{
		Config:     config,
		Assets:     assets,
		MailSender: dialer,
	}
}

func (handler *Handler) OvoHandler() {
	conn, client, err := handler.CreateClient(handler.Config.Ovo.SftpSource)
	if err != nil {
		logrus.Fatalf("Failed to create client: %v", err)
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
	channelName := "Indodana"
	logrus.Printf("Job Running... Indodana")
	conn, client, err := handler.CreateClient(handler.Config.Indodana.SftpSource)
	if err != nil {
		logrus.Printf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
		return
	}

	defer func() {
		client.Close()
		conn.Close()
	}()

	files, err := client.ReadDir(handler.Config.Indodana.SourcePath)
	if err != nil {
		logrus.Fatalf("Failed to read directory: %v", err)
		return
	}

	connDest, clientDest, err := handler.CreateClient(handler.Config.Indodana.SftpDestination)
	if err != nil {
		logrus.Printf("Failed to create client: %v", err)
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}
		return
	}
	defer func() {
		clientDest.Close()
		connDest.Close()
	}()

	for _, file := range files {
		var err error
		if file.IsDir() {
			continue // skip subdirectories
		}

		remoteFile, err := client.Open(handler.Config.Indodana.SourcePath + "/" + file.Name())
		if err != nil {
			logrus.Printf("Failed to open file %s: %v", file.Name(), err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		// info, err := client.Stat(handler.Config.Indodana.SourcePath + "/" + file.Name())
		// if err != nil {
		// 	handler.OnErrorHandler("",channelName, err)
		// 	continue
		// }

		defer remoteFile.Close()

		localPath := handler.Config.TempFolder + "/before/indodana/"

		if err := os.MkdirAll(localPath, 0755); err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		localFile, err := os.Create(localPath + file.Name())
		if err != nil {
			logrus.Printf("Failed to create local file %s: %v", file.Name(), err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		defer localFile.Close()

		_, err = io.Copy(localFile, remoteFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %s: %v", file.Name(), err)
			handler.OnErrorHandler("", channelName, err)
			continue
		} else {
			logrus.Infof("Downloaded:", file.Name())
		}

		f, err := excelize.OpenFile(localFile.Name())
		if err != nil {
			logrus.Fatalf("Got error %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		// var header []string
		var content [][]string

		rows, err := f.GetRows("Ledger")
		if err != nil {
			logrus.Errorf("Got error when get rows %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		var countBefore int
		for idx, each := range rows {
			if idx > 0 {
				countBefore++
			}
			content = append(content, each)
		}

		outputDir := "./tmp/after"

		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			logrus.Errorf("Error when create directory %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}
		arrName := strings.Split(localFile.Name(), "/")

		newFilename := strings.ReplaceAll(arrName[len(arrName)-1], "_yokke-ptp", "")
		newFilename = strings.ReplaceAll(newFilename, ".xlsx", "")

		newFilename = newFilename + ".csv"
		newFile, err := os.Create(outputDir + "/" + newFilename)
		if err != nil {
			logrus.Errorf("Error when create file %v", err)
			handler.OnErrorHandler("", channelName, err)
			continue
		}

		writer := csv.NewWriter(newFile)
		writer.Comma = ';'

		for _, each := range content {
			if err := writer.Write(each); err != nil {
				logrus.Errorf("Failed to write file %v: %v", newFile.Name(), err)
				handler.OnErrorHandler("", channelName, err)
				continue
			}
		}

		writer.Flush()

		fmt.Println("INDODANA file " + localFile.Name() + " converted to ---->  " + newFilename + " successfully")

		dstFile, err := clientDest.Create(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Errorf("Failed to put file %v to sftp server. Err: %", newFilename, err.Error())
		}

		defer dstFile.Close()

		if _, err := newFile.Seek(0, 0); err != nil {
			logrus.Errorf("Failed to seek file %v: %v", newFile.Name(), err)
		}

		_, err = io.Copy(dstFile, newFile)
		if err != nil {
			logrus.Errorf("Failed to copy file %v to sftp server. Err:", newFile.Name(), err.Error())
		}

		// read again to count row after converted
		convertedFile, err := clientDest.Open(handler.Config.Indodana.DestinationPath + "/" + newFilename)
		if err != nil {
			logrus.Fatalf("Failed to read file (converted) %s: %v", newFilename, err)
		}

		defer convertedFile.Close()

		reader := csv.NewReader(convertedFile)
		reader.Comma = ';'
		convertedRecords, err := reader.ReadAll()
		if err != nil {
			logrus.Errorf("Failed to read csv: %v", err)
		}

		var countAfter int
		countAfter = len(convertedRecords) - 1

		logrus.Printf("Count before: %d", countBefore)
		logrus.Printf("Count after: %d", countAfter)

		logrus.Printf("Success converting file")

		// IF NO ERROR

	}

}

func (handler *Handler) OnErrorHandler(reason string, channelName string, err error) {
	// message := gomail.NewMessage()
	// message.SetHeader("From", handler.Config.Smtp.From)
	// message.SetHeader("To", handler.Config.Smtp.To)
	// now := time.Now().Format("2006-01-02 15:04:05")
	// subject := "Proses Konversi Excel ke CSV - " + channelName + " " + now

	// message.SetHeader("Subject", subject)
	// message.SetBody("text/html", body)
}

func (handler *Handler) CreateClient(sftpConfig config.Sftp) (conn *ssh.Client, cl *sftp.Client, e error) {
	var err error
	sshConfig := &ssh.ClientConfig{
		User: sftpConfig.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(sftpConfig.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err = ssh.Dial("tcp", sftpConfig.Host+":"+strconv.Itoa(sftpConfig.Port), sshConfig)
	if err != nil {
		logrus.Fatalf("Failed to dial: %v", err)
		return nil, nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		logrus.Fatalf("Failed to create SFTP client: %v", err)
		return conn, nil, err
	}

	return conn, client, err
}
