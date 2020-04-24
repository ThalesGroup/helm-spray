package log

import (
	"bufio"
	"fmt"
	"github.com/prometheus/common/log"
	"os"
	"strconv"
	"strings"
)

// Log spray messages
func Info(level int, str string, params ...interface{}) {
	var logStr = "[spray] "

	if level == 2 {
		logStr = logStr + "  > "
	} else if level == 3 {
		logStr = logStr + "    o "
	} else if level == 4 {
		logStr = logStr + "      - "
	} else if level >= 5 {
		logStr = logStr + "        . "
	}

	if len(params) != 0 {
		fmt.Println(logStr + fmt.Sprintf(str, params...))
	} else {
		fmt.Println(logStr + str)
	}
}

func WithNumberedLines(level int, str string, params ...interface{}) {
	// Number of lines to be printed
	numberOfLines := strings.Count(str, "\n")
	if len(str) > 0 && !strings.HasSuffix(str, "\n") {
		numberOfLines++
	}

	// Compute the number of digits corresponding to this number of lines, so that the format of eachline is correct
	numberOfDigits := 0
	for numberOfLines != 0 {
		numberOfLines /= 10
		numberOfDigits = numberOfDigits + 1
	}
	format := "[%" + strconv.Itoa(numberOfDigits) + "d] %s"

	// Print line by line
	lineNbr := 0
	scanner := bufio.NewScanner(strings.NewReader(str))
	for scanner.Scan() {
		log.Info(level, fmt.Sprintf(format, lineNbr, scanner.Text()), params)
		lineNbr++
	}
}

// Log error and exit
func ErrorAndExit(str string, params ...interface{}) {
	if len(params) != 0 {
		os.Stderr.WriteString(fmt.Sprintf(str+"\n", params...))
	} else {
		os.Stderr.WriteString(str + "\n")
	}

	os.Exit(1)
}
