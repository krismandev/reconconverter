package mail

import (
	"path"
	"text/template"

	"gopkg.in/gomail.v2"
)

type Sender interface {
	DialAndSend(...*gomail.Message) error
}

type EmailType string

const (
	NotifConverted EmailType = "mail-notif.html"
)

// type EmailType map[string]string
// var views = map[EmailType]string{}

type Assets struct {
	Templates map[EmailType]*template.Template
}

func NewAssets(basePath string, mailTypes ...EmailType) (*Assets, error) {

	templates := make(map[EmailType]*template.Template)
	for _, tp := range mailTypes {
		templatePath := path.Join(basePath, string(tp))

		t := template.New(string(tp))
		t, err := t.ParseFiles(templatePath)
		if err != nil {
			return nil, err
		}
		templates[tp] = t
	}
	return &Assets{
		Templates: templates,
	}, nil
}
