package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/mail"
	"os"
	"path"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

func getGmail() *gmail.Service {
	credentials := path.Join(os.Getenv("HOME"), ".gmail-cli-credentials.json")
	b, err := ioutil.ReadFile(credentials)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	return srv
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := path.Join(os.Getenv("HOME"), ".gmail-cli-token.json")

	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		log.Printf("Error reading token from file: %v", err)
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {

	fromName := flag.String("from-name", "", "From name")
	fromEmail := flag.String("from-email", "", "From email address")
	toName := flag.String("to-name", "", "To name")
	toEmail := flag.String("to-email", "", "To email address")
	subject := flag.String("subject", "", "Subject")

	flag.Parse()

	if *fromEmail == "" {
		log.Fatalf("fromEmail is empty")
	}
	if *toEmail == "" {
		log.Fatalf("toEmail is empty")
	}
	if *fromName == "" {
		fromName = fromEmail
	}
	if *toName == "" {
		toName = toEmail
	}

	from := mail.Address{Name: *fromName, Address: *fromEmail}
	to := mail.Address{Name: *toName, Address: *toEmail}

	header := make(map[string]string)
	header["From"] = from.String()
	header["To"] = to.String()
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"
	header["Subject"] = *subject

	var base64encBuff bytes.Buffer
	encoder := base64.NewEncoder(base64.RawURLEncoding, &base64encBuff)
	for k, v := range header {
		if _, err := fmt.Fprintf(encoder, "%s: %s\r\n", k, v); err != nil {
			log.Fatalf("Failed writing headers: %v", err)
		}
	}

	if _, err := fmt.Fprintf(encoder, "\r\n"); err != nil {
		log.Fatalf("Failed writing header end: %v", err)
	}

	var f *os.File
	var err error
	if len(flag.Args()) > 0 {
		fname := flag.Args()[0]
		if f, err = os.Open(fname); err != nil {
			log.Fatalf("Failed to open file %v: %v", fname, err)
		} else {
			defer f.Close()
		}
	} else {
		f = os.Stdin
	}

	if _, err := io.Copy(encoder, f); err != nil {
		log.Fatalf("Error reading body, err %v", err)
	}

	encoder.Close()

	gmsg := gmail.Message{
		Raw: base64encBuff.String(),
	}

	srv := getGmail()

	user := "me"

	if _, err := srv.Users.Messages.Send(user, &gmsg).Do(); err != nil {
		log.Fatalf("Unable to send msg %v: %v", gmsg, err)
	}
}
