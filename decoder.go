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
	"strings"
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
	br *bufio.Reader
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

// +--------------+
// |MAGIC(48bits) |
// +----------------+
// | Length(64bits) |
// +-------------------------------------------+
// | Version (null-terminated string, 256bits) |
// +-------------------------------------------+
// | Tempo (32bits float32)           |
// +----------------------------------+
// |     track block                  |
// |              ...                 |
func (f *framer) readSplice() (*Pattern, error) {
	var hdr struct {
		MAGIC   [6]byte
		Length  uint64
		Version [32]byte
	}

	err := binary.Read(f.br, binary.BigEndian, &hdr)

	// Do we have something that looks like a valid SPLICE stream?
	if err != nil || !bytes.Equal(hdr.MAGIC[:], []byte("SPLICE")) {
		return nil, ErrNotSPLICE
	}

	p := &Pattern{length: int64(hdr.Length)}

	// Limit how many bytes we'll read from this stream
	r := io.LimitReader(f.br, p.length-32)

	// Read the tempo, which is a little-endian float32
	var tempo struct {
		Value float32
	}
	err = binary.Read(r, binary.LittleEndian, &tempo)

	if err != nil {
		return nil, err
	}

	p.Version = strings.TrimRight(string(hdr.Version[:]), "\x00")
	p.Tempo = tempo.Value

	err = f.readTracks(r, p)

	if err != nil {
		return nil, err
	}

	return p, nil
}

// bytesToBools maps an array of bytes to an array of bools
func (f *framer) bytesToBools(buf []byte) []bool {
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
func (f *framer) readTracks(r io.Reader, p *Pattern) error {
	var hdr struct {
		ID     uint8
		Length uint32
	}
	var steps []byte
	var buf []byte
	for {
		err := binary.Read(r, binary.BigEndian, &hdr)
		if err == io.EOF {
			// no more tracks
			break
		} else if err != nil {
			return err
		}

		if int(hdr.Length) > len(buf) {
			buf = make([]byte, hdr.Length)
		}
		// fmt.Printf("name length: %v\n", nameLen)
		if _, err := r.Read(buf[0:hdr.Length]); err != nil {
			return err
		}
		name := string(buf[0:hdr.Length])

		if steps == nil {
			steps = make([]byte, 16)
		}
		// fmt.Printf("name: %v\n", name)
		if _, err := r.Read(steps); err != nil {
			return err
		}
		track := track{ID: strconv.Itoa(int(hdr.ID)), Name: name, Steps: f.bytesToBools(steps)}
		p.Tracks = append(p.Tracks, track)
	}
	return nil
}
