package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	auth        smtp.Auth
	logFilePath string = "./logs.txt"
	configPath  string = "config.json"
	config      map[string]string
	fullUrl     string
)

type Cards []map[string]interface{}

type Email struct {
	from    string
	to      []string
	body    string
	subject string
}

type templateData struct {
	CardName string
	CardUrl  string
	CardDue  string
}

func main() {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	json.Unmarshal(configFile, &config)
	fullUrl = config["baseUrl"] + config["apiKey"] + config["secret"]
	auth = smtp.PlainAuth("", "trellodev2020", config["password"], "smtp.gmail.com")
	repeat(24*time.Second, getCards)
}

func NewEmail(body string) *Email {
	return &Email{
		from:    "trellodev2020@gmail.com",
		to:      []string{"loiseaubillonlouis@gmail.com"},
		subject: "Rappel tâches trello !",
		body:    body,
	}
}

func getCards() {
	response, err := http.Get(fullUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
	body, readError := ioutil.ReadAll(response.Body)
	if readError != nil {
		log.Fatal(readError)
	}
	var cards Cards
	jsonError := json.Unmarshal(body, &cards)

	if jsonError != nil {
		log.Fatal(jsonError)
	}
	manageCards(cards)
}

func manageCards(cards Cards) {
	loc, _ := time.LoadLocation("Europe/Paris")
	now := time.Now().In(loc)

	for _, card := range cards {
		if card["dueComplete"] == true {
			return
		}
		due, err := time.Parse(time.RFC3339, card["due"].(string))
		if err != nil {
			log.Fatal(err)
		}
		willExpireSoon := due.Sub(now).Hours() <= 24
		if willExpireSoon {
			var data templateData
			data.CardDue = FormatTimeRemaining(now, due)
			data.CardUrl = card["shortUrl"].(string)
			data.CardName = card["name"].(string)
			html := ParseTemplate("template.html", data)
			email := NewEmail(html)
			_, sentError := email.sendEmail()
			if sentError != nil {
				log := fmt.Sprintf("Error sending emails : %v", sentError)
				cslog([]byte(log))
			} else {
				log := fmt.Sprintf("%v Emails sent : %v\n", now, len(email.to))
				cslog([]byte(log))
			}
		}
	}
}

func (e *Email) sendEmail() (bool, error) {
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subject := "Subject: " + e.subject + "\n"
	to := "To: " + strings.Join(e.to, ",") + "\n"
	msg := []byte(to + subject + mime + "\n" + e.body)
	addr := "smtp.gmail.com:587"

	if err := smtp.SendMail(addr, auth, e.from, e.to, msg); err != nil {
		return false, err
	}
	return true, nil
}

func ParseTemplate(file string, data templateData) string {
	t, err := template.ParseFiles(file)
	if err != nil {
		log.Fatal(err)
	}
	buffed := new(bytes.Buffer)
	if err = t.Execute(buffed, data); err != nil {
		log.Fatal(err)
	}
	return buffed.String()
}

func FormatTimeRemaining(a, b time.Time) string {
	if a.Location() != b.Location() {
		b = b.In(a.Location())
	}
	if a.After(b) {
		a, b = b, a
	}
	y1, M1, d1 := a.Date()
	y2, M2, d2 := b.Date()

	h1, m1, s1 := a.Clock()
	h2, m2, s2 := b.Clock()

	year := int(y2 - y1)
	month := int(M2 - M1)
	day := int(d2 - d1)
	hour := int(h2 - h1)
	min := int(m2 - m1)
	sec := int(s2 - s1)

	if sec < 0 {
		sec += 60
		min--
	}
	if min < 0 {
		min += 60
		hour--
	}
	if hour < 0 {
		hour += 24
		day--
	}
	if day < 0 {
		t := time.Date(y1, M1, 32, 0, 0, 0, 0, time.UTC)
		day += 32 - t.Day()
		month--
	}
	if month < 0 {
		month += 12
		year--
	}

	return strconv.Itoa(day) + " jours, " + strconv.Itoa(hour) + " heures, " + strconv.Itoa(min) + " minutes et " + strconv.Itoa(sec) + " secondes."
}

func repeat(duration time.Duration, function func()) {
	for range time.Tick(duration) {
		function()
	}
}

func cslog(data []byte) {
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}
