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

// Formatter should not be instantiated directly as it doesn't have any values set.
// Prefer to use `DefaultFormatter` or `New()` if you need to make changes to it.
type Formatter struct {
	LevelLetters  int
	LevelUpper    bool
	LevelLower    bool
	CompactFull   bool
	CompactSimple bool

	isTerminal bool
	jsonFmt    *prettyjson.Formatter

	sync.Once
}

// DefaultFormatter is a ready to use Formatter for use with logrus.
var DefaultFormatter = New()

// New will allow you to create a new formatter with reasonable defaults to customise.
func New() *Formatter {
	return &Formatter{
		LevelLetters:  3,
		LevelUpper:    true,
		CompactSimple: true,
	}
}

var reCompact = regexp.MustCompile(`\s*\n\s*`)
var bSpace = []byte{' '}
var bNewline = []byte{'\n'}

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

	keySize := 5
	keys := make([]string, 0, len(entry.Data))
	for k := range entry.Data {
		keys = append(keys, k)
		if n := len(k); n > keySize {
			keySize = n
		}
	}
	sort.Strings(keys)
	if keySize > 20 {
		keySize = 20
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

	fmt.Fprintf(b, "%s %s %s%s",
		timeColour(entry.Time.Format("Jan 02 15:04:05.000")),
		levelColour(levelText),
		prefix,
		entry.Message,
	)

	padding := []byte(fmt.Sprintf("\n%s", string(bytes.Repeat([]byte{' '}, keySize+4))))
	for _, key := range keys {
		if (key == "prefix" || key == "rpc" || key == "user") && prefix != "" {
			continue
		}
		value := entry.Data[key]
		data, err := json.Marshal(value)
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

	return b.Bytes(), nil
}
