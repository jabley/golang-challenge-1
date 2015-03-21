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

// frameHeaderLen is the number of bytes required to express the header.
// this does not include the magic marker
const frameHeaderLen = 40

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
	return fr
}

func dump(data []byte) {
	fmt.Printf("%v\n", hex.Dump(data))
}

// +----------------------------------+
// |MAGIC(48bits) |
// | Length(64bits) |
// +----------------------------------+
// | Version (null-terminated string, 256bits)        |
// +----------------------------------+
// |     track block                  |
// |              ...                 |
func (f *framer) readSplice() (*Pattern, error) {
	// a working buffer
	buf := make([]byte, 64)

	_, err := f.br.Read(buf[0:14])

	// Do we have something that looks like a valid SPLICE stream?
	if err != nil || !bytes.Equal(buf[0:6], []byte("SPLICE")) {
		return nil, ErrNotSPLICE
	}

	// how big is the stream?
	size := binary.BigEndian.Uint64(buf[6:14])
	p := &Pattern{length: int64(size)}

	// Limit how many bytes we'll read from this stream
	r := io.LimitReader(f.br, p.length)

	// Read the version string, which is a null-terminated string
	io.ReadFull(r, buf[0:32])
	version := f.readNullTerminatedString(buf[0:32])

	// Read the tempo, which is a little-endian float32
	io.ReadFull(io.LimitReader(r, 4), buf[0:4])
	tempo, err := f.readFloat32(buf[0:4])
	if err != nil {
		return nil, err
	}

	p.Version = version
	p.Tempo = tempo

	err = f.readTracks(buf, r, p)

	if err != nil {
		return nil, err
	}

	return p, nil
}

// bitsToBools maps an array of bytes to an array of bools
func (f *framer) bitsToBools(buf []byte) []bool {
	res := make([]bool, len(buf))
	for i, b := range buf {
		res[i] = (b != 0)
	}
	return res
}

func (f *framer) readFloat32(b []byte) (float32, error) {
	var res float32
	err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &res)
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

// +-----------+
// | id(8bits) |
// +----------------------------------+
// | length (32bits)                  |
// +----------------------------------+
// | name (ascii string, length bytes)|
// +----------------------------------+
// | steps (128bits)                  |
// +----------------------------------+
func (f *framer) readTracks(buf []byte, r io.Reader, p *Pattern) error {
	for {
		_, err := r.Read(buf[0:1])
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		id := int(buf[0])
		// fmt.Printf("id: %v\n", id)
		r.Read(buf[0:4])
		nameLen := binary.BigEndian.Uint32(buf[0:4])
		// fmt.Printf("name length: %v\n", nameLen)
		r.Read(buf[0:nameLen])
		name := string(buf[0:nameLen])
		// fmt.Printf("name: %v\n", name)
		r.Read(buf[0:16])
		steps := f.bitsToBools(buf[0:16])
		track := track{ID: strconv.Itoa(id), Name: name, Steps: steps}
		p.Tracks = append(p.Tracks, track)
	}
	return nil
}
