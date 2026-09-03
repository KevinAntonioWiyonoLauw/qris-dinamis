package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/verssache/qris-dinamis-go/internal/qris"
)

func ask(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("  QRIS Static → Dynamic Converter v2.0")
	fmt.Println("  By: Gidhan")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	qrisString := ask(reader, "[?] Input QRIS string: ")

	validation := qris.Validate(qrisString)
	if !validation.Valid {
		fmt.Println("\n[✗] Invalid QRIS:")
		for _, e := range validation.Errors {
			fmt.Printf("    - %s\n", e)
		}
		os.Exit(1)
	}

	parsed := qris.Parse(qrisString)

	fmt.Println("\n[✓] QRIS Parsed:")
	fmt.Printf("    Merchant : %s\n", parsed.MerchantName)
	fmt.Printf("    City     : %s\n", parsed.MerchantCity)
	fmt.Printf("    Method   : %s\n", parsed.Method)
	if parsed.Currency == "360" {
		fmt.Println("    Currency : IDR")
	} else {
		fmt.Printf("    Currency : %s\n", parsed.Currency)
	}

	if parsed.Method == qris.MethodDynamic {
		if parsed.Amount != "" {
			fmt.Printf("    Amount   : %s\n", parsed.Amount)
		} else {
			fmt.Println("    Amount   : -")
		}
		fmt.Println("\n[!] This QRIS is already dynamic.")
		return
	}

	amountStr := ask(reader, "\n[?] Input nominal (Rupiah): ")
	amount, err := strconv.Atoi(amountStr)
	if err != nil || amount <= 0 {
		fmt.Println("[✗] Invalid amount.")
		os.Exit(1)
	}

	var fee *qris.ConvertFee
	useFee := ask(reader, "[?] Add service fee? (y/n): ")
	if strings.ToLower(useFee) == "y" {
		feeType := ask(reader, "[?] Fixed or Percentage? (f/p): ")
		switch strings.ToLower(feeType) {
		case "f":
			feeVal := ask(reader, "[?] Fee amount (Rupiah): ")
			v, err := strconv.ParseFloat(feeVal, 64)
			if err == nil {
				fee = &qris.ConvertFee{Type: qris.FeeFixed, Value: v}
			}
		case "p":
			feeVal := ask(reader, "[?] Fee percentage: ")
			v, err := strconv.ParseFloat(feeVal, 64)
			if err == nil {
				fee = &qris.ConvertFee{Type: qris.FeePercentage, Value: v}
			}
		}
	}

	opts := qris.ConvertOptions{Amount: strconv.Itoa(amount), Fee: fee}
	result := qris.Convert(qrisString, opts)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("  Result")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("\n%s\n\n", result)
}
