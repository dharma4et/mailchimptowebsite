package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
)

type Configuration struct {
	SmtpHost              string
	SmtpPort              string
	SmtpUsername          string
	SmtpPassword          string
	SmtpFromEmail         string
	SendEmailTo           string
	MailChimpServerPrefix string
	MailChimpApiKey       string
	UrlDayLinkId          string
	UrlDayApiKey          string
}

type UrlDay struct {
	Status int `json:"status"`
	Data   struct {
		Id       string `json:"id"`
		Alias    string `json:"alias"`
		Url      string `json:"url"`
		ShortUrl string `json:"short_url"`
	} `json:"data"`
}

type MailChimpSent struct {
	TotalItems int `json:"total_items"`
	Campaigns  []struct {
		Id         string `json:"id"`
		ArchiveUrl string `json:"archive_url"`
		Status     string `json:"status"`
	} `json:"campaigns"`
}

func main() {
	conf := ReadConfiguration()

	currentUrlDay := GetCurrentUrlDay(conf)
	currentMailchimpUrl := GetLatestMailChimpCampaignUrl(conf)

	logMessage := fmt.Sprintf("Current UrlDay: %s\r\nCurrent MailChimp: %s\r\n", currentUrlDay, currentMailchimpUrl)

	if currentUrlDay != currentMailchimpUrl {
		logMessage = logMessage + "\tUpdate Required"
		UpdateUrlDay(conf, currentMailchimpUrl)
		logMessage = logMessage + "\r\n\tUpdate Successful"
	} else {
		logMessage = logMessage + "\tNO Update Required"
	}

	SendGmailEmail(conf, "[ADMC][SUCCESS] MailChimp To Website Automation", logMessage)
}

func ReadConfiguration() Configuration {
	conf := Configuration{}

	// Assumes there is a .env file in the directory you are executing from which contains:
	/*
		SmtpHost
		SmtpPort
		SmtpUsername
		SmtpPassword
		SmtpFromEmail
		SendEmailTo
		MailChimpServerPrefix
		MailChimpApiKey
		UrlDayLinkId
		UrlDayApiKey
	*/
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	conf.SmtpHost = os.Getenv("SmtpHost")
	conf.SmtpPort = os.Getenv("SmtpPort")
	conf.SmtpUsername = os.Getenv("SmtpUsername")
	conf.SmtpPassword = os.Getenv("SmtpPassword")
	conf.SmtpFromEmail = os.Getenv("SmtpFromEmail")
	conf.SendEmailTo = os.Getenv("SendEmailTo")
	conf.MailChimpServerPrefix = os.Getenv("MailChimpServerPrefix")
	conf.MailChimpApiKey = os.Getenv("MailChimpApiKey")
	conf.UrlDayLinkId = os.Getenv("UrlDayLinkId")
	conf.UrlDayApiKey = os.Getenv("UrlDayApiKey")

	return conf
}

func SendGmailEmail(conf Configuration, emailSubject string, emailBody string) {

	to := []string{conf.SendEmailTo} // TODO - split if comma separated

	message := []byte("Subject: " + emailSubject + "\r\n\r\n" + emailBody)

	// Create authentication
	auth := smtp.PlainAuth("", conf.SmtpFromEmail, conf.SmtpPassword, conf.SmtpHost)

	// Send actual message
	err := smtp.SendMail(conf.SmtpHost+":"+conf.SmtpPort, auth, conf.SmtpFromEmail, to, message)
	if err != nil {
		log.Fatal(err)
	}
}

func GetCurrentUrlDay(conf Configuration) string {
	url := "https://www.urlday.com/api/v1/links/" + conf.UrlDayLinkId

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+conf.UrlDayApiKey)

	resp, err := client.Do(req)
	if err != nil {
		HandleError(conf, err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			HandleError(conf, err)
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)

	// Convert response body to UrlDay struct
	urlday := UrlDay{}
	err = json.Unmarshal(bodyBytes, &urlday)
	if err != nil {
		HandleError(conf, err)
	}

	return urlday.Data.Url
}

func UpdateUrlDay(conf Configuration, urlUpdate string) {

	newUrlInfo := fmt.Sprintf("url=%s", urlUpdate)

	url := "https://www.urlday.com/api/v1/links/" + conf.UrlDayLinkId
	client := &http.Client{}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer([]byte(newUrlInfo)))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+conf.UrlDayApiKey)

	resp, err := client.Do(req)
	if err != nil {
		HandleError(conf, err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			HandleError(conf, err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		e := errors.New("issue with UrlDay update, response status not 200")
		HandleError(conf, e)
	}

}

func GetLatestMailChimpCampaignUrl(conf Configuration) string {
	url := fmt.Sprintf("https://%s.api.mailchimp.com/3.0/campaigns?status=sent&sort_field=send_time&sort_dir=DESC&count=1", conf.MailChimpServerPrefix)

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	req.SetBasicAuth("anystring", conf.MailChimpApiKey)

	resp, err := client.Do(req)
	if err != nil {
		HandleError(conf, err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			HandleError(conf, err)
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)

	// Convert response body to MailChimpSent struct
	mailchimpSent := MailChimpSent{}
	err = json.Unmarshal(bodyBytes, &mailchimpSent)
	if err != nil {
		HandleError(conf, err)
	}

	currentUrl := ""
	if len(mailchimpSent.Campaigns) == 1 {
		currentUrl = mailchimpSent.Campaigns[0].ArchiveUrl
	}

	return currentUrl
}

func HandleError(conf Configuration, e error) {
	SendGmailEmail(conf, "[ADMC][ERROR] with MailChimp to Website Automation", "Error Message: "+e.Error())
	log.Fatal(e)
}
