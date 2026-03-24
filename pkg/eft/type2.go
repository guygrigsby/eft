package eft

// Type2Options holds demographic/descriptive data for a Type-2 record.
type Type2Options struct {
	// IDC is the Information Designation Character (0-99).
	IDC int
	// Fields is a map of field number to value for any Type-2 fields
	// the caller wants to set. Common fields include:
	//   2.018 NAM — name (last{US}first{US}middle)
	//   2.020 POB — place of birth code
	//   2.022 DOB — date of birth YYYYMMDD
	//   2.024 SEX — sex code (M/F/U)
	//   2.025 RAC — race code
	//   2.031 HGT — height in feet/inches (e.g., "510")
	//   2.073 CRI — controlling agency ORI
	Fields map[int][]byte
}

// NewType2Record creates a Type-2 descriptive/demographic record.
// At minimum, the IDC must be set. Additional fields are set via the Fields map.
func NewType2Record(opts Type2Options) *Record {
	r := &Record{Type: 2}

	// 2.001 LEN — computed during encode
	// 2.002 IDC
	r.SetField(2, []byte(formatIDC(opts.IDC)))

	for num, val := range opts.Fields {
		if num == 1 || num == 2 {
			continue // don't override LEN or IDC
		}
		r.SetField(num, val)
	}

	return r
}

func formatIDC(idc int) string {
	if idc < 10 {
		return "0" + string(rune('0'+idc))
	}
	return string(rune('0'+idc/10)) + string(rune('0'+idc%10))
}
