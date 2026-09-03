package qris

import (
	"strconv"
)

// tagNames maps known EMVCo / QRIS tag IDs to human-readable names.
var tagNames = map[string]string{
	"00": "Payload Format Indicator",
	"01": "Point of Initiation Method",
	"02": "Visa",
	"03": "Mastercard",
	"04": "Mastercard",
	"15": "Visa",
	"52": "Merchant Category Code",
	"53": "Transaction Currency",
	"54": "Transaction Amount",
	"55": "Tip or Convenience Indicator",
	"56": "Value of Convenience Fee (Fixed)",
	"57": "Value of Convenience Fee (%)",
	"58": "Country Code",
	"59": "Merchant Name",
	"60": "Merchant City",
	"61": "Postal Code",
	"62": "Additional Data Field",
	"63": "CRC",
}

func init() {
	for i := 26; i <= 51; i++ {
		tagNames[twoDigit(i)] = "Merchant Account Information"
	}
}

func twoDigit(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// isNestedTag reports whether a tag contains nested TLV sub-elements
// (merchant account info tags 26-51 and additional-data tag 62).
func isNestedTag(tag string) bool {
	if tag == "62" {
		return true
	}
	n, err := strconv.Atoi(tag)
	return err == nil && n >= 26 && n <= 51
}

// ParseTLV parses a raw TLV string into elements, mirroring the reference
// implementation: malformed tails are silently dropped.
func ParseTLV(data string) []TLV {
	elements := []TLV{}
	pos := 0
	for pos < len(data) {
		if pos+4 > len(data) {
			break
		}
		tag := data[pos : pos+2]
		length, err := strconv.Atoi(data[pos+2 : pos+4])
		if err != nil || pos+4+length > len(data) {
			break
		}
		value := data[pos+4 : pos+4+length]
		name, ok := tagNames[tag]
		if !ok {
			name = "Unknown (" + tag + ")"
		}
		el := TLV{Tag: tag, Name: name, Length: length, Value: value}
		if isNestedTag(tag) {
			el.Children = ParseTLV(value)
		}
		elements = append(elements, el)
		pos += 4 + length
	}
	return elements
}

// Parse parses a QRIS string into structured Data.
func Parse(qrisString string) Data {
	raw := ParseTLV(qrisString)
	findTag := func(tag string) (TLV, bool) {
		for _, t := range raw {
			if t.Tag == tag {
				return t, true
			}
		}
		return TLV{}, false
	}

	method := MethodStatic
	if v, ok := findTag("01"); ok && v.Value == "12" {
		method = MethodDynamic
	}

	var tipIndicator TipIndicator
	if v, ok := findTag("55"); ok {
		switch v.Value {
		case "01":
			tipIndicator = TipPrompt
		case "02":
			tipIndicator = TipFixed
		case "03":
			tipIndicator = TipPercentage
		}
	}

	merchantAccountInfo := []MerchantAccountInfo{}
	for _, t := range raw {
		n, err := strconv.Atoi(t.Tag)
		if err != nil || n < 26 || n > 51 || t.Children == nil {
			continue
		}
		info := MerchantAccountInfo{Tag: t.Tag, Fields: t.Children}
		for _, c := range t.Children {
			switch c.Tag {
			case "00":
				info.GloballyUniqueID = c.Value
			case "01":
				info.MerchantID = c.Value
			case "02":
				if info.MerchantID == "" {
					info.MerchantID = c.Value
				}
			case "03":
				info.MerchantCriteria = c.Value
			}
		}
		merchantAccountInfo = append(merchantAccountInfo, info)
	}

	strOr := func(tag, def string) string {
		if v, ok := findTag(tag); ok {
			return v.Value
		}
		return def
	}
	amount := ""
	if v, ok := findTag("54"); ok {
		amount = v.Value
	}
	tipFixed := ""
	if v, ok := findTag("56"); ok {
		tipFixed = v.Value
	}
	tipPct := ""
	if v, ok := findTag("57"); ok {
		tipPct = v.Value
	}
	additional := []TLV{}
	if v, ok := findTag("62"); ok && v.Children != nil {
		additional = v.Children
	}
	crc := ""
	if v, ok := findTag("63"); ok {
		crc = v.Value
	}

	return Data{
		Version:              strOr("00", "01"),
		Method:               method,
		MerchantAccountInfo:  merchantAccountInfo,
		MerchantCategoryCode: strOr("52", ""),
		Currency:             strOr("53", "360"),
		Amount:               amount,
		TipIndicator:         tipIndicator,
		TipFixed:             tipFixed,
		TipPercentage:        tipPct,
		CountryCode:          strOr("58", "ID"),
		MerchantName:         strOr("59", ""),
		MerchantCity:         strOr("60", ""),
		PostalCode:           strOr("61", ""),
		AdditionalData:       additional,
		CRC:                  crc,
		Raw:                  raw,
	}
}
