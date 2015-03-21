package drum

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// ErrNotSPLICE is returned if the stream is not a SPLICE format
var ErrNotSPLICE = errors.New("Not a SPLICE stream")

// frameHeaderLen is the number of bytes required to express the header
const frameHeaderLen = 46

type stickyErrReader struct {
	r   io.Reader
	err error
}

func (ser stickyErrReader) Read(p []byte) (n int, err error) {
	if ser.err != nil {
		return 0, ser.err
	}
	n, err = ser.r.Read(p)
	ser.err = err
	return
}

type framer struct {
	br         *bufio.Reader
	headerBuf  [frameHeaderLen]byte
	getReadBuf func(size uint32) []byte
	readBuf    []byte // cache for default getReadBuf
}

// DecodeFile decodes the drum machine file found at the provided path
// and returns a pointer to a parsed pattern which is the entry point to the
// rest of the data.
// TODO: implement
func DecodeFile(path string) (*Pattern, error) {
	// data, err := ioutil.ReadFile(path)
	// dump(data)

	file, err := os.Open(path)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	br := bufio.NewReader(stickyErrReader{r: file})
	framer := newFramer(br)

	return framer.readSplice()
}

func newFramer(br *bufio.Reader) framer {
	fr := framer{br: br}
	fr.getReadBuf = func(size uint32) []byte {
		if cap(fr.readBuf) >= int(size) {
			return fr.readBuf[:size]
		}
		fr.readBuf = make([]byte, size)
		return fr.readBuf
	}
	return fr
}

func dump(data []byte) {
	fmt.Printf("%v\n", hex.Dump(data))
}

// +----------------------------------+
// |MAGIC(48bits) | unused (32bits)   |
// | Length(32bits) |
// +----------------------------------+
// | Version (null-terminated)        |
// +----------------------------------+
// |     track block                  |
// |              ...                 |
func (f *framer) readSplice() (*Pattern, error) {
	magic, err := f.br.Peek(6)

	if err != nil {
		return nil, ErrNotSPLICE
	}

	if !bytes.Equal(magic[:6], []byte("SPLICE")) {
		return nil, ErrNotSPLICE
	}

	// Read header
	p, err := f.readFrameHeader(f.headerBuf[:], f.br)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("data length: %v\n", p.length)
	data := f.getReadBuf(p.length - frameHeaderLen + 14)

	if _, err := io.ReadFull(f.br, data); err != nil {
		fmt.Printf("Error slurping the data: %v\n", err)
		return nil, err
	}

	version := f.readNullTerminatedString(f.headerBuf[14:])
	tempo, err := f.readFloat32(data[0:4])

	if err != nil {
		return nil, err
	}

	p.Version = version
	p.Tempo = tempo

	pos := 4 // skip tempo bytes

	for pos < len(data) {
		// fmt.Printf("Parsing track – pos: %v, size: %v\n", pos, len(data))
		// dump(data[pos:])
		id := int(data[pos])

		// fmt.Printf("Reading track %v\n", id)
		pos += 4 // single byte for ID, plus 3 unused
		nameSize := int(data[pos])
		pos++
		name := string(data[pos : pos+nameSize])
		pos += nameSize
		// fmt.Printf("Parsed name %v\n", name)
		steps := f.bitsToBools(data[pos : pos+16])
		track := track{ID: strconv.Itoa(id), Name: name, Steps: steps}
		// fmt.Printf("Parsed track %v\n", track)
		p.Tracks = append(p.Tracks, track)
		pos += 16
		// fmt.Printf("Added a track\n")
	}

	return &p, nil
}

func (f *framer) readFrameHeader(buf []byte, r io.Reader) (Pattern, error) {
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return Pattern{}, err
	}

	length := f.readUint32(buf[10:14])

	return Pattern{
		length: length,
	}, nil

}

// bitsToBools maps an array of bytes to an array of bools
func (f *framer) bitsToBools(buf []byte) []bool {
	res := make([]bool, len(buf))
	for i, b := range buf {
		res[i] = (b != 0)
	}
	return res
}

func (f *framer) readUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}

func (f *framer) readFloat32(b []byte) (float32, error) {
	var res float32
	buf := bytes.NewReader(b)
	err := binary.Read(buf, binary.LittleEndian, &res)
	return res, err
}

func (f *framer) readNullTerminatedString(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[0:i])
		}
	}
	return ""
}
