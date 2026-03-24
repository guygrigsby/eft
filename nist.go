// Package eft provides types and functions for creating ANSI/NIST-ITL
// (Electronic Fingerprint Transmission) files per the FBI EBTS v11.1
// specification. It supports traditional encoding with Type-1, Type-2,
// and Type-14 records.
package eft

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
)

// ANSI/NIST-ITL separator characters for traditional encoding.
const (
	FS byte = 0x1C // File Separator — separates logical records
	GS byte = 0x1D // Group Separator — separates fields within a record
	RS byte = 0x1E // Record Separator — separates subfields
	US byte = 0x1F // Unit Separator — separates items within a subfield
)

// CompressionAlgorithm identifies the image compression used in a record.
type CompressionAlgorithm string

const (
	CompressionNone  CompressionAlgorithm = "NONE"
	CompressionWSQ   CompressionAlgorithm = "WSQ"
	CompressionJPEGB CompressionAlgorithm = "JPEGB"
	CompressionJPEGL CompressionAlgorithm = "JPEGL"
	CompressionJP2   CompressionAlgorithm = "JP2"
	CompressionJP2L  CompressionAlgorithm = "JP2L"
)

// Field represents a single tagged field in a logical record.
// Tag format is "TT.FFF" (e.g., "1.001", "14.013").
type Field struct {
	// RecordType is the logical record type number (1, 2, 14, etc.).
	RecordType int
	// FieldNumber is the field number within the record.
	FieldNumber int
	// Value holds the field data. For text fields this is the ASCII content.
	// For binary image data (e.g., 14.999) this is raw bytes.
	Value []byte
}

// Tag returns the field identifier string, e.g. "14.013".
func (f Field) Tag() string {
	return fmt.Sprintf("%d.%03d", f.RecordType, f.FieldNumber)
}

// Record is an ordered collection of fields comprising one logical record
// in an ANSI/NIST-ITL transaction.
type Record struct {
	Type   int
	Fields []Field
	// rawBinary holds the complete binary encoding for binary record types
	// (Type-4, Type-7) that don't use tagged field encoding.
	rawBinary []byte
}

// SetField sets or replaces a field in the record. If the field number
// already exists, it is replaced; otherwise it is appended.
func (r *Record) SetField(fieldNum int, value []byte) {
	for i, f := range r.Fields {
		if f.FieldNumber == fieldNum {
			r.Fields[i].Value = value
			return
		}
	}
	r.Fields = append(r.Fields, Field{
		RecordType:  r.Type,
		FieldNumber: fieldNum,
		Value:       value,
	})
}

// GetField returns the value of a field by number, or nil if not found.
func (r *Record) GetField(fieldNum int) []byte {
	for _, f := range r.Fields {
		if f.FieldNumber == fieldNum {
			return f.Value
		}
	}
	return nil
}

// encode serializes the record into its wire format. Binary record types
// (Type-4, Type-7) return their raw binary directly. Tagged record types
// use traditional ANSI/NIST-ITL encoding with auto-computed LEN.
func (r *Record) encode() ([]byte, error) {
	// Binary record types have pre-built raw bytes.
	if r.rawBinary != nil {
		return r.rawBinary, nil
	}

	// Sort fields by field number, but 001 (LEN) will be computed last.
	sorted := make([]Field, len(r.Fields))
	copy(sorted, r.Fields)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FieldNumber < sorted[j].FieldNumber
	})

	// First pass: encode without LEN to measure size.
	// LEN field format: "TT.001:NNNN" where NNNN is the total byte count
	// including the LEN field itself and the trailing FS.
	lenTag := fmt.Sprintf("%d.001:", r.Type)

	var buf bytes.Buffer
	first := true
	for _, f := range sorted {
		if f.FieldNumber == 1 {
			continue // skip LEN, we'll compute it
		}
		if !first {
			buf.WriteByte(GS)
		}
		first = false
		buf.WriteString(f.Tag())
		buf.WriteByte(':')
		buf.Write(f.Value)
	}
	buf.WriteByte(FS) // record terminator

	// Compute total length: LEN field + GS + rest
	// LEN field = tag + length_digits
	// We need to solve: total = len(lenTag) + digits(total) + 1(GS) + buf.Len()
	// where digits(total) is the number of decimal digits in total.
	bodyLen := buf.Len()
	if !first {
		// There are other fields, so LEN will be followed by GS
		bodyLen += 1 // for GS between LEN field and next field
	}
	baseLen := len(lenTag) + bodyLen

	// Iteratively determine digit count since it affects total length.
	totalLen := baseLen
	for {
		digitLen := len(strconv.Itoa(totalLen))
		candidate := baseLen + digitLen
		if candidate == totalLen {
			break
		}
		totalLen = candidate
	}

	// Build final output.
	var out bytes.Buffer
	out.WriteString(lenTag)
	out.WriteString(strconv.Itoa(totalLen))
	if !first {
		out.WriteByte(GS)
	}
	out.Write(buf.Bytes())

	return out.Bytes(), nil
}

// Transaction represents a complete ANSI/NIST-ITL transaction file,
// composed of one or more logical records.
type Transaction struct {
	Records []*Record
}

// AddRecord appends a record to the transaction.
func (t *Transaction) AddRecord(r *Record) {
	t.Records = append(t.Records, r)
}

// Encode serializes the entire transaction to traditional ANSI/NIST-ITL
// binary format. It updates the Type-1 CNT field (1.003) to reflect all
// records in the transaction, then encodes each record sequentially.
func (t *Transaction) Encode() ([]byte, error) {
	if len(t.Records) == 0 {
		return nil, fmt.Errorf("eft: transaction has no records")
	}

	// Find the Type-1 record and update CNT (1.003).
	var type1 *Record
	for _, r := range t.Records {
		if r.Type == 1 {
			type1 = r
			break
		}
	}
	if type1 == nil {
		return nil, fmt.Errorf("eft: transaction missing mandatory Type-1 record")
	}

	// Build CNT value: first subfield is count of records,
	// subsequent subfields are "type{US}IDC" pairs.
	// IDC (Information Designation Character) is 0 for Type-1, then sequential.
	var cntBuf bytes.Buffer
	cntBuf.WriteString(strconv.Itoa(len(t.Records)))
	idc := 0
	for _, r := range t.Records {
		cntBuf.WriteByte(RS)
		cntBuf.WriteString(strconv.Itoa(r.Type))
		cntBuf.WriteByte(US)
		if r.Type == 1 {
			cntBuf.WriteString("0")
		} else {
			fmt.Fprintf(&cntBuf, "%02d", idc)
			idc++
		}
	}
	type1.SetField(3, cntBuf.Bytes())

	// Encode all records.
	var out bytes.Buffer
	for _, r := range t.Records {
		data, err := r.encode()
		if err != nil {
			return nil, fmt.Errorf("eft: encoding record type %d: %w", r.Type, err)
		}
		out.Write(data)
	}
	return out.Bytes(), nil
}
