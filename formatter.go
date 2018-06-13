package formatrus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/hokaccha/go-prettyjson"
	"github.com/mgutz/ansi"
	"github.com/norganna/depict"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	green   = ansi.ColorFunc("green")
	yellow  = ansi.ColorFunc("yellow")
	red     = ansi.ColorFunc("red")
	blue    = ansi.ColorFunc("blue")
	cyan    = ansi.ColorFunc("cyan")
	magenta = ansi.ColorFunc("magenta")
	whiteH  = ansi.ColorFunc("magenta+h")
	blackH  = ansi.ColorFunc("black+h")
)

func noColour(s string) string {
	return s
}

func braketise(s string) string {
	return fmt.Sprintf("[%s]", s)
}

type sorter struct {
	order map[string]int
	pri   map[string]int
	keys  []string
}

func (s *sorter) Len() int {
	return len(s.keys)
}

func (s *sorter) Swap(i, j int) {
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}

func (s *sorter) Less(i, j int) bool {
	a := s.keys[i]
	b := s.keys[j]
	if s.order != nil {
		ai := s.order[a]
		bi := s.order[b]
		if ai < bi {
			return false
		}
		if ai > bi {
			return true
		}
	}
	if s.pri != nil {
		ap := s.pri[a]
		bp := s.pri[b]
		if ap > bp {
			return false
		}
		if ap < bp {
			return true
		}
	}
	return strings.Compare(a, b) < 1
}

// Formatter should not be instantiated directly as it doesn't have any values set.
// Prefer to use `DefaultFormatter` or `New()` if you need to make changes to it.
type Formatter struct {
	// LevelLetters denotes the number of letters to show for the level (from 1 to 5).
	LevelLetters int
	// LevelUpper sets whether to UPPERCASE the level text.
	LevelUpper bool
	// LevelLower sets whether to lowercase the level text.
	LevelLower bool
	// CompactFull makes all json structures take a single line.
	CompactFull bool
	// CompactSimple causes any short json structures to be compact, but larger ones will still be block indented.
	CompactSimple bool
	// MessageAfter places the message text on a new line after any data.
	MessageAfter bool
	// CompactMessage allows short messages without any data lines to be placed on the log line
	CompactMessage bool
	// ParagraphAll adds a newline after any log line.
	ParagraphAll bool
	// ParagraphBlock adds a newline after any log line block (multi-line log messages)
	ParagraphBlock bool
	// Ordering provides a priority order for data keys (higher numbers appear earlier, < 0 come after unprioritised)
	Ordering map[string]int

	isTerminal bool
	jsonFmt    *prettyjson.Formatter

	sync.Once
}

// DefaultFormatter is a ready to use Formatter for use with logrus.
var DefaultFormatter = New()

// New will allow you to create a new formatter with reasonable defaults to customise.
func New() *Formatter {
	return &Formatter{
		LevelLetters:   3,
		LevelUpper:     true,
		CompactSimple:  true,
		MessageAfter:   true,
		CompactMessage: true,
	}
}

var reCompact = regexp.MustCompile(`\s*\n\s*`)
var bSpace = []byte{' '}
var bNewline = []byte{'\n'}

// Order adds a priority to a given list of keys (chainable call).
func (f *Formatter) Order(priority int, keys ...string) *Formatter {
	if f.Ordering == nil {
		f.Ordering = map[string]int{}
	}

	for _, key := range keys {
		f.Ordering[key] = priority
	}

	return f
}

// Format takes a logrus Entry and renders it into a byte slice.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	f.Do(func() {
		if entry.Logger != nil {
			switch v := entry.Logger.Out.(type) {
			case *os.File:
				f.isTerminal = terminal.IsTerminal(int(v.Fd()))
			}
		}
		f.jsonFmt = prettyjson.NewFormatter()
		f.jsonFmt.Indent = 1
	})

	var levelColour func(string) string
	var levelText string
	var levelText3 string
	var levelText5 string
	switch entry.Level {
	case logrus.InfoLevel:
		levelColour = green
		levelText3 = "Inf"
		levelText5 = "Info "
	case logrus.WarnLevel:
		levelColour = yellow
		levelText3 = "Wrn"
		levelText5 = "Warn "
	case logrus.ErrorLevel:
		levelColour = red
		levelText3 = "Err"
		levelText5 = "Error"
	case logrus.FatalLevel:
		levelColour = red
		levelText3 = "Ftl"
		levelText5 = "Fatal"
	case logrus.PanicLevel:
		levelColour = red
		levelText3 = "Pnc"
		levelText5 = "Panic"
	default:
		levelColour = blue
		levelText3 = "Dbg"
		levelText5 = "Debug"
	}

	dataColour := cyan
	prefixColour := magenta
	userColour := whiteH
	timeColour := blackH

	if !f.isTerminal {
		levelColour = noColour
		dataColour = noColour
		prefixColour = noColour
		userColour = noColour
		timeColour = braketise
	}

	b := entry.Buffer
	if b == nil {
		b = &bytes.Buffer{}
	}

	if f.LevelLetters <= 0 {
		f.LevelLetters = 3
	}

	if f.LevelLetters >= 5 {
		levelText = levelText5
	} else if f.LevelLetters > 3 {
		levelText = levelText5[0:f.LevelLetters]
	} else {
		levelText = levelText3[0:f.LevelLetters]
	}

	if f.LevelUpper {
		levelText = strings.ToUpper(levelText)
	}
	if f.LevelLower {
		levelText = strings.ToUpper(levelText)
	}

	user := ""
	prefix := ""

	if v, ok := entry.Data["user"]; ok {
		if v, ok := v.(string); ok {
			user = userColour(v + "@")
		}
	}
	if v, ok := entry.Data["rpc"]; ok {
		if v, ok := v.(string); ok {
			prefix = v
		}
	}
	if v, ok := entry.Data["prefix"]; ok {
		if v, ok := v.(string); ok {
			if prefix != "" {
				v = v + "/"
			}
			prefix = v + prefix
		}
	}
	if prefix != "" {
		prefix = prefixColour(prefix + ":")
	}
	prefix = user + prefix
	if prefix != "" {
		prefix += " "
	}

	fmt.Fprintf(b, "%s %s",
		timeColour(entry.Time.Format("Jan 02 15:04:05.000")),
		levelColour(levelText),
	)

	if prefix != "" {
		fmt.Fprintf(b, " %s", prefix)
	}

	var orders []string

	keySize := 5
	keys := make([]string, 0, len(entry.Data))
	for key, v := range entry.Data {
		if key == "_order" {
			orders = v.([]string)
			continue
		}
		if (key == "prefix" || key == "rpc" || key == "user") && prefix != "" {
			continue
		}
		keys = append(keys, key)
		if n := len(key); n > keySize {
			keySize = n
		}
	}

	if f.Ordering == nil && len(orders) == 0 {
		sort.Strings(keys)
	} else {
		var pri map[string]int
		if len(orders) > 0 {
			pri = map[string]int{}
			for i, k := range orders {
				pri[k] = i
			}
		}

		s := &sorter{
			order: f.Ordering,
			keys:  keys,
			pri:   pri,
		}
		sort.Sort(s)
	}

	if keySize > 20 {
		keySize = 20
	}

	// We can cuddle if we haven't been told to put the message after, or if we've been told we can cuddle, and there's
	// no keys to print and the message isn't overly long.
	cuddleMessage := !f.MessageAfter || (f.CompactMessage && len(keys) == 0 && len(entry.Message) < 100)
	if cuddleMessage {
		fmt.Fprint(b, entry.Message)
	}

	padding := []byte(fmt.Sprintf("\n%s", string(bytes.Repeat([]byte{' '}, keySize+4))))
	for _, key := range keys {
		value := entry.Data[key]

		data, err := json.Marshal(depict.Portray(value))

		if err == nil && len(data) == 2 && data[0] == '{' {
			if v, ok := value.(error); ok {
				str := v.Error()
				if len(str) > 0 {
					data, err = json.Marshal(str)
				}
			} else if v, ok := value.(fmt.Stringer); ok {
				str := v.String()
				if len(str) > 0 {
					data, err = json.Marshal(str)
				}
			}
		}

		if err == nil && f.isTerminal {
			if pretty, pErr := f.jsonFmt.Format(data); pErr == nil {
				data = pretty
			}
		}

		if err != nil {
			data = []byte(fmt.Sprintf("%#v", data))
		}

		if f.isTerminal {
			l := keySize - len(key)
			b.Write(bNewline)
			fmt.Fprintf(b, "  %s: ", dataColour(key))
			b.Write(bytes.Repeat(bSpace, l))
			if f.CompactFull || (f.CompactSimple && len(data) < 100) {
				b.Write(reCompact.ReplaceAll(data, bSpace))
			} else {
				b.Write(bytes.Replace(data, bNewline, padding, -1))
			}
		} else {
			fmt.Fprintf(b, "  %s=", key)
			b.Write(data)
		}
	}
	b.Write(bNewline)

	if !cuddleMessage {
		fmt.Fprintf(b, "  %s\n", entry.Message)
		if f.ParagraphAll || f.ParagraphBlock {
			b.Write(bNewline)
		}
	} else if f.ParagraphAll {
		b.Write(bNewline)
	}

	return b.Bytes(), nil
}
