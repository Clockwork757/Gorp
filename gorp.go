// It's Gorp!
package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/docopt/docopt-go"
	"github.com/gookit/color"
)

var lock = &sync.Mutex{}

func syncPrint(s string) {
	defer lock.Unlock()
	lock.Lock()

	fmt.Println(s)
}

type meta struct {
	folderMode  bool
	invertMatch bool
	abs         bool
	colorFunc   func(string) string
}

func setupBuilder(path string, options meta) *strings.Builder {
	var builder *strings.Builder
	if options.folderMode {
		var relPath string
		if options.abs {
			relPath, _ = filepath.Abs(".")
		} else {
			relPath = "."
		}
		relPath = filepath.Join(relPath, path)

		builder = &strings.Builder{}
		builder.WriteString(relPath)
	}
	return builder
}

// Opens a file at path path
// Returns the stdin file pointer if it's /dev/stdin
// If you open /dev/stdin like a normal file it will close after one new-line
// I feel like there's a better way to resolve this issue but I'm not sure
func open(path string) (*os.File, error) {
	var f *os.File
	var err error
	if path != "/dev/stdin" {
		f, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(getFileContentType(f), "text/plain") {
			return nil, errors.New("Non text file contents")
		}
	} else {
		f = os.Stdin
	}
	return f, err
}

func handleLine(text string, re *regexp.Regexp, lineNumber int, options meta) string {
	text = options.colorFunc(text)
	if options.folderMode {
		fmt.Sprintf("\n[%s]:%s", color.Green.Render(lineNumber), text)
	}
	return text
}

// Opens the file and loops over it
// Prints each line as its processed or stores it in a string.Builder if in folder mode
func handleFile(path string, re *regexp.Regexp, wg *sync.WaitGroup, options meta) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		f, err := open(path)
		if err != nil {
			return
		}

		lineNumber := 0
		match := false

		builder := setupBuilder(path, options)

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lineNumber++
			text := scanner.Text()
			if re.MatchString(text) {
				match = true
				line := handleLine(text, re, lineNumber, options)
				if options.folderMode {
					builder.WriteString(line)
				} else {
					print(line)
				}
			}
		}
		if match && options.folderMode {
			syncPrint(builder.String())
		}
	}()
}

func getFileContentType(f *os.File) string {
	buffer := make([]byte, 512)
	_, err := f.Read(buffer)
	if err != nil {
		return ""
	}
	f.Seek(0, 0)
	return http.DetectContentType(buffer)
}

func buildColorFunc(col string, re *regexp.Regexp, noColor bool, IsTerminal bool) func(string) string {
	if noColor || !IsTerminal {
		return func(t string) string { return t }
	}
	return func(t string) string {
		return re.ReplaceAllStringFunc(t, func(s string) string {
			if color.IsDefinedTag(col) {
				return color.ApplyTag(col, s)
			}
			return color.ApplyTag("red", s)
		})
	}
}

func main() {
	usage := `Gorp.
Usage:
  gorp [-hvc color --abs-path --no-color] <pattern> [<file>]
Arguments:
  <pattern>     Regex pattern to search for
  <file>        Optional file or folder [default:stdin]
Options:
  -v            Invert Matches
  -c=color      Highlight color, color tag or rbg code [default:red]
  --no-color    Turn off highlighting
  --abs-path    Print all paths absolutely
  -h --help     Show this screen.`
	opts, err := docopt.ParseDoc(usage)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	rawRe, _ := opts.String("<pattern>")

	re, err := regexp.Compile(rawRe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fileName, _ := opts.String("<file>")
	stat, err := os.Stat(fileName)
	var isDir bool
	if err != nil {
		isDir = false
	} else {
		isDir = stat.IsDir()
	}
	invert, _ := opts.Bool("-v")
	abs, _ := opts.Bool("--abs-path")
	noColor, _ := opts.Bool("--no-color")
	col, _ := opts.String("-c")
	col = strings.ToLower(col)
	IsTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

	options := meta{
		folderMode:  isDir && IsTerminal,
		invertMatch: invert,
		abs:         abs,
		colorFunc:   buildColorFunc(col, re, noColor, IsTerminal),
	}

	wg := &sync.WaitGroup{}

	if fileName != "" {
		filepath.Walk(fileName, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not access %s\n", path)
				return nil
			}
			if info.IsDir() {
				return nil
			}
			handleFile(path, re, wg, options)
			return nil
		})
	} else {
		handleFile("/dev/stdin", re, wg, options)
	}
	wg.Wait()
}
