package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/gabriel-vasile/mimetype/internal/scan"
)

type line struct {
	fields  int
	hasMore bool
}

func eq(l1, l2 []line) bool {
	if len(l1) != len(l2) {
		return false
	}
	for i := range l1 {
		if l1[i].fields != l2[i].fields || l1[i].hasMore != l2[i].hasMore {
			return false
		}
	}

	return true
}

var testcases = []struct {
	name    string
	csv     string
	comma   byte
	comment byte
}{{
	"empty", "", ',', '#',
}, {
	"simple",
	`foo,bar,baz
1,2,3
"1","a",b`,
	',', '#',
}, {
	"crlf line endings",
	"foo,bar,baz\r\n1,2,3\r\n",
	',', '#',
}, {
	"leading and trailing space",
	`1, abc ,3`,
	',', '#',
}, {
	"empty quote",
	`1,"",3`,
	',', '#',
}, {
	"quotes with comma",
	`1,",",3`,
	',', '#',
}, {
	"quotes with quote",
	`1,""",3`,
	',', '#',
}, {
	"fewer fields",
	`foo,bar,baz
1,2`,
	',', '#',
}, {
	"more fields",
	`1,2,3,4`,
	',', '#',
}, {
	"forgot quote",
	`1,"Forgot,3`,
	',', '#',
}, {
	"unescaped quote",
	`1,"abc"def",3`,
	',', '#',
}, {
	"unescaped quote",
	`1,"abc"def",3`,
	',', '#',
}, {
	"unescaped quote2",
	`1,abc"quote"def,3`,
	',', '#',
}, {
	"escaped quote",
	`1,abc""def,3`,
	',', '#',
}, {
	"new line",
	`1,abc
def,3`,
	',', '#',
}, {
	"new line quotes",
	`1,"abc
def",3`,
	',', '#',
}, {
	"quoted field at end",
	`1,"abc"`,
	',', '#',
}, {
	"not ended quoted field at end",
	`1,"abc`,
	',', '#',
}, {
	"empty field",
	`1,,3`,
	',', '#',
}, {
	"unicode fields",
	`💁,👌,🎍,😍`,
	',', '#',
}, {
	"comment",
	`#comment`,
	',', '#',
}, {
	"line with \\r at the end",
	"123\r\n456\r",
	',', '#',
}}

func TestParser(t *testing.T) {
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expected, _ := stdlibLines(tc.csv, tc.comma, tc.comment)
			got := ourLines(tc.csv, tc.comma, tc.comment)
			if !eq(expected, got) {
				t.Errorf("\n%s\n expected: %v got: %v", tc.csv, expected, got)
			}
		})
	}
}

func ourLines(data string, comma, comment byte) []line {
	p := NewParser(comma, comment, scan.Bytes(data))
	lines := []line{}
	for {
		fields, hasMore := p.CountFields()
		if !hasMore {
			break
		}
		lines = append(lines, line{fields, hasMore})
	}
	return lines
}

// stdlibLines returns the []line records obtained using the stdlib CSV parser.
func stdlibLines(data string, comma, comment byte) ([]line, error) {
	if comma > unicode.MaxASCII || comment > unicode.MaxASCII {
		return nil, fmt.Errorf("comma or comment not ASCII")
	}

	if strings.IndexByte(data, 0) != -1 {
		return nil, fmt.Errorf("CSV contains null byte 0x00")
	}
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = rune(comma)
	r.ReuseRecord = true
	r.FieldsPerRecord = -1 // we don't care about lines having same number of fields
	r.LazyQuotes = true
	r.Comment = rune(comment)

	var err error
	lines := []line{}
	for {
		l, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, line{len(l), err != io.EOF})
	}
	return lines, err
}

var sample = `
1,2,3
"a", "b", "c"
a,b,c`

func BenchmarkCSVStdlibDecoder(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d := csv.NewReader(strings.NewReader(sample))
		for {
			_, err := d.Read()
			if err == io.EOF {
				break
			}
		}
	}
}
func BenchmarkCSVOurParser(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := NewParser(',', '#', scan.Bytes(sample))
		for {
			_, hasMore := p.CountFields()
			if !hasMore {
				break
			}
		}
	}
}

func FuzzParser(f *testing.F) {
	for _, p := range testcases {
		f.Add(p.csv, byte(','), byte('#'))
	}
	f.Fuzz(func(t *testing.T, data string, comma, comment byte) {
		expected, err := stdlibLines(data, comma, comment)
		// The sddlib CSV parser can accept UTF8 runes for comma and comment.
		// Our parser does not need that functionality, so it returns different
		// results for UTF8 inputs. Skip fuzzing when  the generated data is UTF8.
		if err != nil {
			t.Skipf("not testable: %v", err)
		}
		got := ourLines(data, comma, comment)
		if !eq(got, expected) {
			t.Errorf("input: %v, comma: %v, comment: %v\n got: %v, expected: %v", data, string(rune(comma)), string(rune(comment)), got, expected)
		}
	})
}
