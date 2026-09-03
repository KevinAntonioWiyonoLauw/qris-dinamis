package qris

import (
	"strconv"
	"strings"
)

// buildTLVString rebuilds a QRIS string from TLV elements (without CRC),
// recomputing the 2-digit length header of every element in the process.
func buildTLVString(elements []TLV) string {
	var b strings.Builder
	for _, el := range elements {
		value := el.Value
		if len(el.Children) > 0 {
			value = buildTLVString(el.Children)
		}
		b.WriteString(el.Tag)
		b.WriteString(twoDigit(len(value)))
		b.WriteString(value)
	}
	return b.String()
}

func makeTLV(tag, value string) TLV {
	return TLV{Tag: tag, Name: "", Length: len(value), Value: value}
}

// Convert transforms a static QRIS string into a dynamic one by injecting
// a transaction amount and an optional service fee, then recalculating the
// CRC16 checksum. Mirrors the reference implementation exactly:
//   - tag 01 is rewritten to "12" (dynamic),
//   - amount (54) and fee tags (55/56/57) are inserted just before tag 58,
//   - pre-existing 54/55/56/57/63 tags are dropped.
func Convert(qrisString string, options ConvertOptions) string {
	elements := ParseTLV(qrisString)

	result := []TLV{}
	amountInserted := false

	managed := map[string]bool{"54": true, "55": true, "56": true, "57": true, "63": true}

	for _, el := range elements {
		if managed[el.Tag] {
			continue
		}
		if el.Tag == "01" {
			result = append(result, makeTLV("01", "12"))
			continue
		}
		if el.Tag == "58" && !amountInserted {
			amount := options.Amount
			if amount == "" {
				amount = "0"
			}
			result = append(result, makeTLV("54", amount))
			if options.Fee != nil {
				if options.Fee.Type == FeeFixed {
					result = append(result, makeTLV("55", "02"))
					result = append(result, makeTLV("56", formatFee(options.Fee.Value)))
				} else {
					result = append(result, makeTLV("55", "03"))
					result = append(result, makeTLV("57", formatFee(options.Fee.Value)))
				}
			}
			amountInserted = true
		}
		result = append(result, el)
	}

	crcInput := buildTLVString(result) + "6304"
	return crcInput + CalculateCRC16(crcInput)
}

// formatFee renders a fee value as TypeScript's Number.prototype.toString()
// does: integral values lose the decimal point, fractional ones keep a
// minimal representation (e.g. 1000 -> "1000", 2.5 -> "2.5").
func formatFee(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
