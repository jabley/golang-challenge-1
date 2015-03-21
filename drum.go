// Package drum is supposed to implement the decoding of .splice drum machine files.
// See golang-challenge.com/go-challenge1/ for more information
package drum

import "fmt"

// Pattern is the high level representation of the
// drum pattern contained in a .splice file.
type Pattern struct {
	Version string
	Tempo   float32
	Tracks  []track
	length  int64
}

func (p *Pattern) String() string {
	res := fmt.Sprintf("Saved with HW Version: %v\nTempo: %v\n", p.Version, p.Tempo)

	for _, track := range p.Tracks {
		res += track.String()
	}
	return res

}

type track struct {
	ID    string
	Name  string
	Steps []bool
}

func (t *track) String() string {
	return "(" + t.ID + ") " + t.Name + "\t" + t.FormatSteps() + "\n"
}

func (t *track) FormatSteps() string {
	res := "|"

	for i, s := range t.Steps {

		if s {
			res += "x"
		} else {
			res += "-"
		}
		if (i+1)%4 == 0 {
			res += "|"
		}
	}

	return res
}
