// Package bgl reads the structure of the simulator's BGL files: the header, the sections and
// subsections, and the records inside them. It does not interpret the records yet.
//
// The layout is the one of the Flight Simulator scenery files (FSX onwards): a 0x38-byte header,
// then a table of sections (each with a type and the subsections of it), subsections pointing at
// runs of records, and every record starting with a 16-bit id and a 32-bit size.
package bgl

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	magic1     = 0x19920201
	magic2     = 0x08051803
	headerSize = 0x38
)

// Section is one section of the file.
type Section struct {
	Type        uint32
	Subsections []Subsection
}

// Subsection is a run of records.
type Subsection struct {
	Records []Record
}

// Record is one record: its id, and where its data is in the file (the size includes the 6-byte
// record header).
type Record struct {
	ID     uint16
	Size   uint32
	Offset uint32
}

// File is a parsed BGL.
type File struct {
	Sections []Section
	data     []byte
}

// Bytes returns the data of a record without its 6-byte header.
func (f *File) Bytes(r Record) []byte {
	start := int(r.Offset) + 6
	end := int(r.Offset) + int(r.Size)
	if start > end || end > len(f.data) {
		return nil
	}
	return f.data[start:end]
}

// Parse reads the structure of a BGL file.
func Parse(data []byte) (*File, error) {
	if len(data) < headerSize {
		return nil, errors.New("too short for a BGL header")
	}
	le := binary.LittleEndian
	if le.Uint32(data[0:4]) != magic1 {
		return nil, fmt.Errorf("not a BGL file (signature %08x)", le.Uint32(data[0:4]))
	}
	if le.Uint32(data[0x10:0x14]) != magic2 {
		return nil, fmt.Errorf("unexpected second signature %08x", le.Uint32(data[0x10:0x14]))
	}
	n := int(le.Uint32(data[0x14:0x18]))
	if n < 0 || n > 4096 {
		return nil, fmt.Errorf("%d sections is not believable", n)
	}
	f := &File{data: data}
	off := headerSize
	for i := 0; i < n; i++ {
		if off+0x14 > len(data) {
			return nil, errors.New("section table runs past the end of the file")
		}
		s := Section{Type: le.Uint32(data[off:])}
		subCount := int(le.Uint32(data[off+8:]))
		subOff := int(le.Uint32(data[off+12:]))
		off += 0x14
		if subCount < 0 || subCount > 1<<20 || subOff < 0 || subOff+subCount*0x10 > len(data) {
			return nil, fmt.Errorf("section %d: subsection table is out of range", i)
		}
		for j := 0; j < subCount; j++ {
			p := subOff + j*0x10
			recCount := int(le.Uint32(data[p+8:]))
			recOff := int(le.Uint32(data[p+12:]))
			sub := Subsection{}
			if recCount < 0 || recCount > 1<<24 || recOff < 0 || recOff > len(data) {
				return nil, fmt.Errorf("section %d subsection %d: records out of range", i, j)
			}
			// records are walked by their sizes: the "count" of the table is for the subsection,
			// but records of some types hold their children inside their own size
			pos := recOff
			for k := 0; k < recCount && pos+6 <= len(data); k++ {
				id := le.Uint16(data[pos:])
				size := le.Uint32(data[pos+2:])
				if size < 6 || pos+int(size) > len(data) {
					break
				}
				sub.Records = append(sub.Records, Record{ID: id, Size: size, Offset: uint32(pos)})
				pos += int(size)
			}
			s.Subsections = append(s.Subsections, sub)
		}
		f.Sections = append(f.Sections, s)
	}
	return f, nil
}
