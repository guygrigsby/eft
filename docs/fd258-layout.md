# FD-258 Card Layout and Crop Regions

## Card Overview

The FBI FD-258 is a standard 8"×8" fingerprint card. The upper portion contains demographic fields; the lower portion has labeled boxes for fingerprint impressions.

This library accepts a scan of the **entire card** including the demographic header. The crop coordinates account for the full card layout — the header area (~28% of the card height) is simply ignored during cropping.

## Physical Layout

```
┌──────────────────────────────────────────────────┐
│  Row 1: Rolled prints (right hand)               │
│  [R.Thumb] [R.Index] [R.Middle] [R.Ring] [R.Little] │
│                                                   │
│  Row 2: Rolled prints (left hand)                │
│  [L.Thumb] [L.Index] [L.Middle] [L.Ring] [L.Little] │
│                                                   │
│  Row 3: Flat/slap prints                         │
│  [Left 4 Fingers] [L.Thumb] [R.Thumb] [Right 4 Fingers] │
└──────────────────────────────────────────────────┘
```

## Fractional Crop Regions

All coordinates use `FractionalRect` — values from 0.0 to 1.0 relative to the **full card** dimensions (including header). This makes the layout resolution-independent.

```go
type FractionalRect struct {
    X1, Y1, X2, Y2 float64  // all in range [0.0, 1.0]
}
```

### Header area (Y: 0.00–0.36)

The demographic header plus transition rows (Leave Blank, Employer and Address, Reason Fingerprinted, Signature) occupy the top ~36% of the card (~2.9" of 8"). This area is not cropped — it is simply skipped by the fingerprint crop regions.

### Row 1: Right hand rolled (Y: 0.36–0.54)

| Finger | Position | X1   | Y1   | X2   | Y2   |
|--------|----------|------|------|------|------|
| R.Thumb  | 1 | 0.02 | 0.36 | 0.20 | 0.54 |
| R.Index  | 2 | 0.20 | 0.36 | 0.40 | 0.54 |
| R.Middle | 3 | 0.40 | 0.36 | 0.60 | 0.54 |
| R.Ring   | 4 | 0.60 | 0.36 | 0.80 | 0.54 |
| R.Little | 5 | 0.80 | 0.36 | 0.98 | 0.54 |

### Row 2: Left hand rolled (Y: 0.565–0.765)

| Finger | Position | X1   | Y1   | X2   | Y2   |
|--------|----------|------|------|------|------|
| L.Thumb  | 6 | 0.02 | 0.565 | 0.20 | 0.765 |
| L.Index  | 7 | 0.20 | 0.565 | 0.40 | 0.765 |
| L.Middle | 8 | 0.40 | 0.565 | 0.60 | 0.765 |
| L.Ring   | 9 | 0.60 | 0.565 | 0.80 | 0.765 |
| L.Little | 10 | 0.80 | 0.565 | 0.98 | 0.765 |

### Row 3: Flat/slap prints (Y: 0.79–0.97)

| Print | Position | X1   | Y1   | X2   | Y2   |
|-------|----------|------|------|------|------|
| Left 4 fingers  | 14 | 0.02 | 0.79 | 0.37 | 0.97 |
| Left thumb       | 15 (part) | 0.37 | 0.79 | 0.50 | 0.97 |
| Right thumb      | 15 (part) | 0.50 | 0.79 | 0.63 | 0.97 |
| Right 4 fingers | 13 | 0.63 | 0.79 | 0.98 | 0.97 |

Note: Both thumbs are combined into position 15 (`FingerBothThumbs`) for the Type-14 slap record.

## Minimum Image Size

`CropFD258()` requires at least 100×100 pixels. Images smaller than this are rejected. For good print quality, scan at 500 DPI (yielding ~4000×4000 pixels for the card).

## Custom Layouts

Override `DefaultFD258Layout()` by constructing your own `FD258Layout` with adjusted `FractionalRect` values for non-standard cards or cards with alignment issues.
