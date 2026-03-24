# Sources and References

## Specifications

| Document | Description | Location |
|----------|-------------|----------|
| **EBTS v11.1** | FBI Electronic Biometric Transmission Specification. Primary reference for record formats, field definitions, and transaction structure. | `EBTS v11.1_Final_508.pdf` (repo root) and [fbibiospecs.cjis.gov](https://www.fbibiospecs.cjis.gov/EBTS/Approved) |
| **NIST SP 500-290** | ANSI/NIST-ITL 1-2011 Update 2015. The underlying standard that EBTS implements. Covers traditional encoding, tagged/binary records, separator hierarchy. | [nvlpubs.nist.gov](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.500-290e3.pdf) |
| **WSQ Spec v3.1** | Wavelet Scalar Quantization compression specification for grayscale fingerprint imagery. | [fbibiospecs.cjis.gov](http://www.fbibiospecs.cjis.gov/Document/Get?fileName=WSQ_Gray-scale_Specification_Version_3_1_Final.pdf) |

## Software References

| Software | Description | URL |
|----------|-------------|-----|
| **NIST NBIS** | NIST Biometric Image Software — reference C implementation of ANSI/NIST-ITL, WSQ, PCASYS, and other biometric tools. Public domain. | [nist.gov/nbis](https://www.nist.gov/services-resources/software/nist-biometric-image-software-nbis) |
| **jtejido/go-wsq** | Pure Go port of the NBIS WSQ codec. Used as default WSQ compressor. v0.0.3-beta. No explicit license (original NBIS is public domain). | [github.com/jtejido/go-wsq](https://github.com/jtejido/go-wsq) |

## ATF eForms Research

ATF does not publish an official EFT file specification. The ATF-specific constants (TOT, DAI, ORI, VER, RFP) were determined from:

1. **EFTSuite** ([eftsuite.com](https://www.eftsuite.com/)) — Commercial tool for creating EFT files. Documentation and support forums describe expected field values.
2. **Community EFT implementations** — Open-source tools on GitHub that generate ATF-compatible EFT files, including field mappings and validation requirements.
3. **ATF eForms validation feedback** — Error messages returned by ATF's upload system when incorrect values are used, documenting required fields and formats.
4. **NFA community forums** — Discussion of successful submissions and required file formats.

## Test Data Sources

Public-domain ANSI/NIST-ITL sample files for testing and validation:

| Source | Description | License |
|--------|-------------|---------|
| **NBIS test data** | AN2K test files and WSQ images from NIST Image Group (NIGOS) | Public domain |
| **NIST Standard References** | Traditional encoding reference transactions | Public domain |
| **bentedesco/eft-fingerprint-viewer** | FD-258 format `.eft` files | MIT |

## Key Design Decisions and Their Sources

| Decision | Source/Rationale |
|----------|-----------------|
| VER `0200` (not `0502`) | ATF eForms requires 2000-era format, not 2011 |
| WSQ at 0.75 bitrate | FBI standard per WSQ Spec v3.1 for 500 ppi |
| Type-4 for rolled, Type-14 for slaps | EBTS v11.1 §5.4 and §5.14; matches EFTSuite output |
| Fixed fractional crop regions | FD-258 has standardized box layout; CV unnecessary |
| `Compressor` interface pattern | go-wsq has no stated license; interface allows swap |
| 12 MB file size limit | ATF eForms upload validation |
| Name format `Last,First Middle` | EBTS field 2.018 NAM specification |
