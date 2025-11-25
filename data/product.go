package data

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Product struct {
	Row        int
	Name       string
	Location   string
	Packaging  string
	Quantity   int
	Expiration time.Time
	SentDays   []int
}

type ExpiringProduct struct {
	Product
	DaysUntil int
	Threshold int
}

func ParseProducts(rows [][]any) []Product {
	var products []Product
	for i, row := range rows {
		if i == 0 || len(row) < 5 {
			continue
		}
		qty, _ := strconv.Atoi(fmt.Sprint(row[3]))
		exp, err := time.Parse("2006-01-02", fmt.Sprint(row[4]))
		if err != nil {
			continue
		}
		var sentDays []int
		if len(row) >= 6 && row[5] != nil {
			sentDays = ParseSentDays(fmt.Sprint(row[5]))
		}
		products = append(products, Product{
			Row:        i + 1,
			Name:       fmt.Sprint(row[0]),
			Packaging:  fmt.Sprint(row[1]),
			Location:   fmt.Sprint(row[2]),
			Quantity:   qty,
			Expiration: exp,
			SentDays:   sentDays,
		})
	}
	return products
}

func ParseSentDays(s string) []int {
	if s == "" {
		return nil
	}
	var days []int
	for part := range strings.SplitSeq(s, ",") {
		if d, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			days = append(days, d)
		}
	}
	return days
}

func FormatSentDays(days []int) string {
	var parts []string
	for _, d := range days {
		parts = append(parts, strconv.Itoa(d))
	}
	return strings.Join(parts, ",")
}
