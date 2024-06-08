package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/config"
	"github.com/avast/retry-go"
)

type Expensor struct {
	rules        []api.Rule
	mailClient   *gmail.Service
	sheetsClient *sheets.Service
	spreadSheet  *sheets.Spreadsheet
	msgChan      chan *api.TransactionDetails
}

func NewExpensor(client *http.Client, cfg *config.Config) (*Expensor, error) {
	ctx := context.Background()
	mailClient, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %v", err)
	}

	sheetsClient, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Sheets service: %v", err)
	}

	e := &Expensor{
		rules:        cfg.Rules,
		mailClient:   mailClient,
		sheetsClient: sheetsClient,
		msgChan:      make(chan *api.TransactionDetails),
	}

	spreadsheet, err := e.createSheet(ctx, cfg.SheetTitle, cfg.SheetID, cfg.SheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create new spreadsheet: %v", err)
	}

	e.spreadSheet = spreadsheet
	return e, nil
}

func (e *Expensor) Read(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			log.Println("Starting evaluation of rules...")
			var wg sync.WaitGroup
			for _, rule := range e.rules {
				wg.Add(1)
				go e.runRule(&wg, rule)
			}
			wg.Wait()
		}
	}

}

func (e *Expensor) createSheet(ctx context.Context, sheetTitle, sheetId, sheetName string) (*sheets.Spreadsheet, error) {
	var spreadsheet *sheets.Spreadsheet
	spreadsheet, err := e.sheetsClient.Spreadsheets.Get(sheetId).Context(ctx).Do()
	if err != nil {
		// If sheet doesn't exist, create a new one
		spreadsheet, err = e.sheetsClient.Spreadsheets.Create(&sheets.Spreadsheet{
			Properties: &sheets.SpreadsheetProperties{
				Title: sheetTitle,
			},
		}).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to create spreadsheet: %v", err)
		}

		// Write headers to the new sheet
		headerRange := fmt.Sprintf("%s!A1:C1", sheetName)
		headerReq := sheets.ValueRange{
			Values: [][]interface{}{
				{"Date/Time", "Expense", "Amount"},
			},
		}
		_, err = e.sheetsClient.Spreadsheets.Values.Update(spreadsheet.SpreadsheetId, headerRange, &headerReq).
			ValueInputOption("RAW").Do()
		if err != nil {
			return nil, fmt.Errorf("failed to write headers to the sheet: %v", err)
		}

		fmt.Println("Created new spreadsheet with title:", sheetTitle)
	} else {
		fmt.Println("Found existing spreadsheet with title:", sheetTitle)
	}

	return spreadsheet, nil
}

func (e *Expensor) Write(ctx context.Context) {
	log.Println("Starting writer...")
	go func() {
		if err := e.write(ctx); err != nil {
			log.Fatalf("Failed to write transaction details: %v", err)
		}
	}()
}

func (e *Expensor) write(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case td := <-e.msgChan:
			// Prepare values to write to the sheet
			var values [][]interface{}
			values = append(values, []interface{}{td.Timestamp, td.MerchantInfo, td.Amount})

			// Write values to the sheet
			writeRange := "Sheet1!A2:C2" // Append to existing data (starting from second row)
			writeReq := sheets.ValueRange{
				Values: values,
			}
			err := retry.Do(
				func() error {
					_, err := e.sheetsClient.Spreadsheets.Values.Append(e.spreadSheet.SpreadsheetId, writeRange, &writeReq).
						ValueInputOption("RAW").InsertDataOption("INSERT_ROWS").Do()
					if err != nil {
						return err
					}

					return nil
				},
				retry.RetryIf(func(err error) bool {
					log.Printf("Error: %+v", err)
					if e, ok := err.(*googleapi.Error); ok {
						if e.Code == 429 {
							log.Printf("Retry func: err is 429")
							return true
						}
					}
					return false
				}),
				retry.Attempts(2),
				retry.Delay(60*time.Second),
				retry.LastErrorOnly(true),
			)

			if err != nil {
				return err
			}

			fmt.Println("Transaction details written to Google Sheet successfully.")
		}

	}
}

func (e *Expensor) runRule(wg *sync.WaitGroup, rule api.Rule) {
	defer wg.Done()
	user := "me"
	// Call the Gmail API to retrieve the list of matching emails
	resp, err := e.mailClient.Users.Messages.List(user).Q(rule.Query).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	// Check if there are any matching emails
	fmt.Println("Total:", len(resp.Messages))
	if len(resp.Messages) == 0 {
		fmt.Println("No matching emails found.")
	}

	// Print the list of matching email subject lines and bodies
	fmt.Println("Matching emails:")
	for _, message := range resp.Messages {
		// Call the Gmail API to retrieve the email details
		msg, err := e.mailClient.Users.Messages.Get(user, message.Id).Do()
		if err != nil {
			log.Fatalf("Unable to retrieve message details: %v", err)
		}

		// Extract subject
		var subject string
		for _, header := range msg.Payload.Headers {
			if header.Name == "Subject" {
				subject = header.Value
				break
			}
		}

		// Extract body
		var body string
		for _, part := range msg.Payload.Parts {
			if part.MimeType == "text/html" {
				bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					log.Printf("Unable to decode message body: %v", err)
				} else {
					body = string(bodyBytes)
				}
				break
			}
		}
		receivedTime := time.Unix(msg.InternalDate/1000, 0)
		dets := ExtractTransactionDetails(body, rule.Amount, rule.MerchantInfo, receivedTime)
		fmt.Println("Subject:", subject)
		// fmt.Println("Body:", body)
		fmt.Println("Details:", dets)
		fmt.Println("-------------------------------------")
		e.msgChan <- dets
		// Mark the message as read
		// _, err = e.mailClient.Users.Messages.Modify(user, msg.Id, &gmail.ModifyMessageRequest{
		// 	RemoveLabelIds: []string{"UNREAD"},
		// }).Do()
		// if err != nil {
		// 	log.Fatalf("Unable to mark message as read: %v", err)
		// }
	}
}

// ExtractTransactionDetails extracts transaction details from the email body
func ExtractTransactionDetails(emailBody string, amountRegex, merchantRegex *regexp.Regexp, receivedTime time.Time) *api.TransactionDetails {

	// Extract transaction details
	amountMatches := amountRegex.FindStringSubmatch(emailBody)
	merchantMatches := merchantRegex.FindStringSubmatch(emailBody)

	// Create TransactionDetails struct
	transaction := &api.TransactionDetails{}

	if len(amountMatches) > 0 {
		// Remove commas and convert to int
		amountStr := strings.ReplaceAll(amountMatches[1], ",", "")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err == nil {
			transaction.Amount = amount
		}
	}

	// Parse timestamp with full date
	transaction.Timestamp = receivedTime.Format("Monday, January 2, 2006, 3:04:05 PM")

	if len(merchantMatches) > 0 {
		transaction.MerchantInfo = merchantMatches[1]
	}

	return transaction
}
