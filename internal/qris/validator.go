package qris

import "strings"

// Validate checks a QRIS string's structural correctness, mirroring the
// reference implementation's error messages and ordering exactly.
func Validate(qrisString string) ValidationResult {
	if len(strings.TrimSpace(qrisString)) == 0 {
		return ValidationResult{Valid: false, Errors: []string{"QRIS string is empty"}}
	}
	str := strings.TrimSpace(qrisString)
	errors := []string{}

	if !strings.HasPrefix(str, "000201") {
		errors = append(errors, `QRIS must start with Payload Format Indicator "000201"`)
	}

	if len(str) < 20 {
		errors = append(errors, "QRIS string is too short")
		return ValidationResult{Valid: false, Errors: errors}
	}

	dataWithoutCRC := str[:len(str)-4]
	declaredCRC := strings.ToUpper(str[len(str)-4:])
	calculatedCRC := CalculateCRC16(dataWithoutCRC)
	if declaredCRC != calculatedCRC {
		errors = append(errors, "CRC mismatch: expected "+calculatedCRC+", got "+declaredCRC)
	}

	elements := ParseTLV(str)
	if len(elements) == 0 {
		errors = append(errors, "Failed to parse any TLV elements")
		return ValidationResult{Valid: false, Errors: errors}
	}

	tags := map[string]bool{}
	for _, e := range elements {
		tags[e.Tag] = true
	}

	requiredTags := []struct{ tag, name string }{
		{"00", "Payload Format Indicator"},
		{"01", "Point of Initiation Method"},
		{"52", "Merchant Category Code"},
		{"53", "Transaction Currency"},
		{"58", "Country Code"},
		{"59", "Merchant Name"},
		{"60", "Merchant City"},
		{"63", "CRC"},
	}
	for _, req := range requiredTags {
		if !tags[req.tag] {
			errors = append(errors, "Missing required tag "+req.tag+" ("+req.name+")")
		}
	}

	for _, e := range elements {
		if e.Tag == "01" && e.Value != "11" && e.Value != "12" {
			errors = append(errors, `Invalid Point of Initiation Method: "`+e.Value+`" (must be "11" or "12")`)
			break
		}
	}

	hasMerchant := false
	for _, e := range elements {
		if isMerchantTag(e.Tag) {
			hasMerchant = true
			break
		}
	}
	if !hasMerchant {
		errors = append(errors, "No Merchant Account Information found (tags 26-51)")
	}

	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

func isMerchantTag(tag string) bool {
	n := 0
	for _, r := range tag {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 26 && n <= 51
}
