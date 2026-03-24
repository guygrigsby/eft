package eft

import (
	"fmt"
	"time"
)

// Type1Options holds the parameters needed to build a Type-1 header record.
type Type1Options struct {
	// TransactionType is the TOT code (e.g., "CAR", "CNA", "FAUF").
	TransactionType string
	// DestinationAgency is the 9-byte ORI of the receiving agency (1.007 DAI).
	DestinationAgency string
	// OriginatingAgency is the 9-byte ORI of the sending agency (1.008 ORI).
	OriginatingAgency string
	// ControlNumber is a unique transaction identifier, 10-40 bytes (1.009 TCN).
	ControlNumber string
	// ControlReference is the TCN of a referenced prior transaction (1.010 TCR). Optional.
	ControlReference string
	// NativeScanningResolution (1.011 NSR), e.g. "19.69" for 500 ppi. Default "00.00".
	NativeScanningResolution string
	// NominalTransmittingResolution (1.012 NTR), e.g. "19.69". Default "00.00".
	NominalTransmittingResolution string
	// DomainName (1.013 DOM), e.g. "NORAM" for North America.
	DomainName string
	// DomainVersion (1.013 DOM), e.g. "11.1".
	DomainVersion string
	// Date overrides the transaction date (1.005 DAT). Defaults to now.
	Date time.Time
}

// NewType1Record creates a Type-1 header record per EBTS v11.1.
// The CNT field (1.003) is populated later by Transaction.Encode.
func NewType1Record(opts Type1Options) (*Record, error) {
	if opts.TransactionType == "" {
		return nil, fmt.Errorf("eft: Type1Options.TransactionType is required")
	}
	if opts.DestinationAgency == "" {
		return nil, fmt.Errorf("eft: Type1Options.DestinationAgency is required")
	}
	if opts.OriginatingAgency == "" {
		return nil, fmt.Errorf("eft: Type1Options.OriginatingAgency is required")
	}
	if opts.ControlNumber == "" {
		return nil, fmt.Errorf("eft: Type1Options.ControlNumber is required")
	}

	r := &Record{Type: 1}

	// 1.001 LEN — computed during encode
	// 1.002 VER — version of the ANSI/NIST-ITL standard
	r.SetField(2, []byte("0502")) // ANSI/NIST-ITL 1-2011 Update 2015

	// 1.003 CNT — populated by Transaction.Encode
	r.SetField(3, []byte("0"))

	// 1.004 TOT
	r.SetField(4, []byte(opts.TransactionType))

	// 1.005 DAT — YYYYMMDD
	date := opts.Date
	if date.IsZero() {
		date = time.Now()
	}
	r.SetField(5, []byte(date.Format("20060102")))

	// 1.006 PRY — priority (default 4)
	r.SetField(6, []byte("4"))

	// 1.007 DAI
	r.SetField(7, []byte(opts.DestinationAgency))

	// 1.008 ORI
	r.SetField(8, []byte(opts.OriginatingAgency))

	// 1.009 TCN
	r.SetField(9, []byte(opts.ControlNumber))

	// 1.010 TCR (optional)
	if opts.ControlReference != "" {
		r.SetField(10, []byte(opts.ControlReference))
	}

	// 1.011 NSR
	nsr := opts.NativeScanningResolution
	if nsr == "" {
		nsr = "00.00"
	}
	r.SetField(11, []byte(nsr))

	// 1.012 NTR
	ntr := opts.NominalTransmittingResolution
	if ntr == "" {
		ntr = "00.00"
	}
	r.SetField(12, []byte(ntr))

	// 1.013 DOM
	if opts.DomainName != "" {
		domValue := opts.DomainName
		if opts.DomainVersion != "" {
			domValue += string(US) + opts.DomainVersion
		}
		r.SetField(13, []byte(domValue))
	}

	return r, nil
}
