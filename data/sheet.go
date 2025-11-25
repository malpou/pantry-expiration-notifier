package data

import (
	"context"
	"fmt"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetClient struct {
	service       *sheets.Service
	spreadsheetID string
	sheetName     string
}

func NewSheetClient(ctx context.Context, credentials []byte, spreadsheetID, sheetName string) (*SheetClient, error) {
	srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentials))
	if err != nil {
		return nil, fmt.Errorf("create sheets client: %w", err)
	}

	return &SheetClient{
		service:       srv,
		spreadsheetID: spreadsheetID,
		sheetName:     sheetName,
	}, nil
}

func (c *SheetClient) LoadProducts() ([]Product, error) {
	resp, err := c.service.Spreadsheets.Values.Get(c.spreadsheetID, c.sheetName+"!A:F").Do()
	if err != nil {
		return nil, fmt.Errorf("read sheet: %w", err)
	}

	return ParseProducts(resp.Values), nil
}

type SentDaysUpdate struct {
	Row      int
	SentDays []int
}

func (c *SheetClient) UpdateSentDays(updates []SentDaysUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	data := make([]*sheets.ValueRange, len(updates))
	for i, u := range updates {
		data[i] = &sheets.ValueRange{
			Range:  fmt.Sprintf("%s!F%d", c.sheetName, u.Row),
			Values: [][]any{{FormatSentDays(u.SentDays)}},
		}
	}

	batchReq := &sheets.BatchUpdateValuesRequest{
		ValueInputOption: "RAW",
		Data:             data,
	}

	_, err := c.service.Spreadsheets.Values.BatchUpdate(c.spreadsheetID, batchReq).Do()
	if err != nil {
		return fmt.Errorf("update sheet: %w", err)
	}

	return nil
}
