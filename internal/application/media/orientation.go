package media

import (
	"encoding/binary"
	"image"
	"image/color"
)

// normalizeImageOrientation applies the JPEG EXIF Orientation tag before crop
// coordinates are evaluated. Browsers honor this tag while Go's image/jpeg
// decoder intentionally returns the stored pixel matrix unchanged; without
// normalization, crop coordinates calculated from a browser preview can map to
// a rectangle with the opposite aspect ratio on the server.
func normalizeImageOrientation(raw []byte, format string, src image.Image) image.Image {
	if format != "jpeg" {
		return src
	}
	orientation := jpegEXIFOrientation(raw)
	if orientation < 2 || orientation > 8 {
		return src
	}
	return transformOrientation(src, orientation)
}

func jpegEXIFOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(data); {
		if data[offset] != 0xff {
			offset++
			continue
		}
		marker := data[offset+1]
		offset += 2
		if marker == 0xd9 || marker == 0xda {
			break
		}
		if marker == 0x00 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		if offset+2 > len(data) {
			break
		}
		segmentLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLength < 2 || offset+segmentLength > len(data) {
			break
		}
		segment := data[offset+2 : offset+segmentLength]
		if marker == 0xe1 && len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
			if orientation := tiffOrientation(segment[6:]); orientation != 0 {
				return orientation
			}
		}
		offset += segmentLength
	}
	return 1
}

func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(data[2:4]) != 42 {
		return 0
	}
	ifdOffset := uint64(order.Uint32(data[4:8]))
	if ifdOffset+2 > uint64(len(data)) {
		return 0
	}
	entryCount := uint64(order.Uint16(data[ifdOffset : ifdOffset+2]))
	entriesStart := ifdOffset + 2
	if entryCount > (uint64(len(data))-entriesStart)/12 {
		return 0
	}
	for i := uint64(0); i < entryCount; i++ {
		entryOffset := entriesStart + i*12
		entry := data[entryOffset : entryOffset+12]
		if order.Uint16(entry[0:2]) != 0x0112 { // Orientation
			continue
		}
		if order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) < 1 { // SHORT
			return 0
		}
		orientation := int(order.Uint16(entry[8:10]))
		if orientation >= 1 && orientation <= 8 {
			return orientation
		}
		return 0
	}
	return 0
}

type orientedImage struct {
	src         image.Image
	orientation int
	bounds      image.Rectangle
}

func transformOrientation(src image.Image, orientation int) image.Image {
	width, height := src.Bounds().Dx(), src.Bounds().Dy()
	if orientation >= 5 {
		width, height = height, width
	}
	return &orientedImage{src: src, orientation: orientation, bounds: image.Rect(0, 0, width, height)}
}

func (img *orientedImage) ColorModel() color.Model { return img.src.ColorModel() }
func (img *orientedImage) Bounds() image.Rectangle { return img.bounds }
func (img *orientedImage) At(x, y int) color.Color {
	if !image.Pt(x, y).In(img.bounds) {
		return color.NRGBA{}
	}
	sourceBounds := img.src.Bounds()
	width, height := sourceBounds.Dx(), sourceBounds.Dy()
	var sx, sy int
	switch img.orientation {
	case 2:
		sx, sy = width-1-x, y
	case 3:
		sx, sy = width-1-x, height-1-y
	case 4:
		sx, sy = x, height-1-y
	case 5:
		sx, sy = y, x
	case 6:
		sx, sy = y, height-1-x
	case 7:
		sx, sy = width-1-y, height-1-x
	case 8:
		sx, sy = width-1-y, x
	default:
		sx, sy = x, y
	}
	return img.src.At(sourceBounds.Min.X+sx, sourceBounds.Min.Y+sy)
}
