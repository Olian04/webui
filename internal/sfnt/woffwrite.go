package sfnt

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"math"
)

// WriteWOFF is the font as a WOFF 1.0 file: the tables of Write, each deflated when that makes it smaller. It is a browser's font format, and the
// standard library has everything it needs, where WOFF2 would want Brotli. This function is not part of the library this package is from.
func (sfnt *SFNT) WriteWOFF() ([]byte, error) {
	otf := sfnt.Write()
	if len(otf) < 12 {
		return nil, fmt.Errorf("no font to write")
	}
	numTables := int(binary.BigEndian.Uint16(otf[4:]))
	if len(otf) < 12+16*numTables {
		return nil, fmt.Errorf("the table directory is cut short")
	}

	type table struct {
		tag                       []byte
		checksum, origLen, offset uint32
		data                      []byte
	}
	tables := make([]table, numTables)
	for i := range tables {
		rec := otf[12+16*i:]
		t := table{tag: rec[0:4], checksum: binary.BigEndian.Uint32(rec[4:]), offset: binary.BigEndian.Uint32(rec[8:]), origLen: binary.BigEndian.Uint32(rec[12:])}
		if uint64(t.offset)+uint64(t.origLen) > uint64(len(otf)) {
			return nil, fmt.Errorf("the table %q is outside the file", t.tag)
		}
		raw := otf[t.offset : t.offset+t.origLen]
		var packed bytes.Buffer
		zw, err := zlib.NewWriterLevel(&packed, zlib.BestCompression)
		if err != nil {
			return nil, err
		}
		if _, err := zw.Write(raw); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		t.data = raw
		if packed.Len() < len(raw) {
			t.data = packed.Bytes()
		}
		tables[i] = t
	}

	// Everything is a few kilobytes at most, and a font that is not is refused here, so that the sizes below fit the fields of the format.
	if len(otf) > math.MaxInt32/2 || numTables > math.MaxUint16 {
		return nil, fmt.Errorf("the font is too large for a WOFF file")
	}
	head := make([]byte, 44+20*numTables) // the header and the table directory, which are written as the tables are placed
	copy(head, "wOFF")
	copy(head[4:], otf[0:4])                                 // the flavor: the version of the font that was wrapped
	binary.BigEndian.PutUint16(head[12:], uint16(numTables)) //nolint:gosec // G115: bounded above
	binary.BigEndian.PutUint32(head[16:], uint32(len(otf)))  //nolint:gosec // G115: bounded above
	var body []byte
	for i, t := range tables {
		for (len(head)+len(body))%4 != 0 {
			body = append(body, 0)
		}
		dir := head[44+20*i:]
		copy(dir, t.tag)
		binary.BigEndian.PutUint32(dir[4:], uint32(len(head)+len(body))) //nolint:gosec // G115: bounded above
		binary.BigEndian.PutUint32(dir[8:], uint32(len(t.data)))         //nolint:gosec // G115: bounded above
		binary.BigEndian.PutUint32(dir[12:], t.origLen)
		binary.BigEndian.PutUint32(dir[16:], t.checksum)
		body = append(body, t.data...)
	}
	for (len(head)+len(body))%4 != 0 {
		body = append(body, 0)
	}
	binary.BigEndian.PutUint32(head[8:], uint32(len(head)+len(body))) //nolint:gosec // G115: bounded above
	out := make([]byte, 0, len(head)+len(body))
	return append(append(out, head...), body...), nil
}
