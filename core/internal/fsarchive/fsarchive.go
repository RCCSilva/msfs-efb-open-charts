// Package fsarchive reads the ".fsarchive" containers the simulator keeps its packages in
// (MSFS 2024: %APPDATA%\Microsoft Flight Simulator 2024\Packages\StreamedPackages\<package>\content).
//
// The container is plain: a 32-byte header, a JSON index of the files (path, offset, size) and
// then the files one after the other. The packages seen so far are not encrypted and not
// compressed; a container that says otherwise is refused, not guessed at.
//
// It only reads. Nothing here writes to the simulator's files.
package fsarchive

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Entry is one file in the container.
type Entry struct {
	Path             string `json:"path"` // with the backslashes of Windows, as stored
	ByteOffset       int64  `json:"byteOffset"`
	ByteSize         int64  `json:"byteSize"`
	UncompressedSize int64  `json:"uncompressed_size"`
}

// Name returns the path with forward slashes.
func (e Entry) Name() string { return strings.ReplaceAll(e.Path, `\`, "/") }

// Archive is an open container.
type Archive struct {
	f       *os.File
	base    int64 // where the data of the files starts
	Entries []Entry
}

type index struct {
	Encryption struct {
		Scheme string `json:"scheme"`
	} `json:"encryptionSetup"`
	Files []Entry `json:"fileInfoList"`
}

// Open opens a container and reads its index.
func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := parse(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

func parse(f *os.File) (*Archive, error) {
	var head [32]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return nil, err
	}
	if string(head[:4]) != "RASA" {
		return nil, errors.New("not a simulator package archive (no RASA signature)")
	}
	// bytes 4..8 version, 8..12 and 12..16: the second says how many bytes the index takes
	// (including a terminator), as the files were seen.
	indexLen := int64(binary.LittleEndian.Uint32(head[12:16]))
	if indexLen <= 0 || indexLen > 64<<20 {
		return nil, fmt.Errorf("the index size %d is not believable", indexLen)
	}
	raw := make([]byte, indexLen)
	if _, err := io.ReadFull(f, raw); err != nil {
		return nil, err
	}
	raw = []byte(strings.TrimRight(string(raw), "\x00"))
	var ix index
	if err := json.Unmarshal(raw, &ix); err != nil {
		return nil, fmt.Errorf("the index is not JSON: %w", err)
	}
	if ix.Encryption.Scheme != "notEncrypted" {
		return nil, fmt.Errorf("archive scheme %q is not supported (only notEncrypted)", ix.Encryption.Scheme)
	}
	for _, e := range ix.Files {
		if e.ByteSize != e.UncompressedSize {
			return nil, fmt.Errorf("%s is compressed, which is not supported", e.Path)
		}
	}
	return &Archive{f: f, base: 32 + indexLen, Entries: ix.Files}, nil
}

// Read returns the bytes of one file.
func (a *Archive) Read(e Entry) ([]byte, error) {
	if e.ByteSize < 0 || e.ByteSize > 1<<30 {
		return nil, errors.New("file size out of range")
	}
	buf := make([]byte, e.ByteSize)
	if _, err := a.f.ReadAt(buf, a.base+e.ByteOffset); err != nil {
		return nil, err
	}
	return buf, nil
}

// Close closes the container.
func (a *Archive) Close() error { return a.f.Close() }
