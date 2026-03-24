# ANSI/NIST-ITL Traditional Encoding

## Separator Hierarchy

Traditional encoding uses four ASCII separator characters to structure data:

| Separator | Hex  | Name                | Separates             |
|-----------|------|---------------------|-----------------------|
| US        | 0x1F | Unit Separator      | Information items      |
| RS        | 0x1E | Record Separator    | Subfields             |
| GS        | 0x1D | Group Separator     | Fields within a record |
| FS        | 0x1C | File Separator      | Records (logical)      |

Field tags use the format `T.FFF:` where T is the record type and FFF is the field number. Example: `1.002:0502` sets the version in a Type-1 record.

## LEN Field Computation

Every tagged record starts with field X.001 (LEN) — the total byte length of the encoded record including the LEN field itself. This creates a circular dependency: the LEN value affects its own digit count.

The library resolves this iteratively:
1. Encode all fields except LEN
2. Estimate LEN digit count
3. Recompute if the digit count changes the total length
4. Converges in 1-2 iterations

See `nist.go:Record.encode()` for the implementation.

## CNT Field (1.003)

The transaction header (Type-1) field 1.003 lists all records in the transaction as type/IDC pairs separated by RS. It's auto-computed by `Transaction.Encode()`.

Format: `1.003:1{RS}0{RS}14{RS}1{RS}14{RS}2{GS}` (Type-1 IDC 0, Type-14 IDC 1, Type-14 IDC 2...)

## Binary vs Tagged Records

Most records (Type-1, 2, 10, 14, etc.) use tagged field encoding. Type-4 and Type-7 use fixed binary headers — no field tags, no separators.

The `Record` struct has a `rawBinary []byte` field. When set, `encode()` returns the raw bytes directly instead of building tagged fields. This is how Type-4 records bypass the tagged encoding path.

## Record Termination

Each tagged record ends with FS (0x1C). The last record in a transaction also ends with FS.

## References

- NIST SP 500-290 §7 (Traditional Encoding)
- EBTS v11.1 §2.3 (Logical Record Structure)
