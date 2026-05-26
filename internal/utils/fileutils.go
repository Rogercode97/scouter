package utils

import (
	"bufio"
	"os"
	"strings"
)

// ExtractLines reads a file and returns the content between startLine and endLine (inclusive, 1-indexed).
func ExtractLines(filePath string, startLine, endLine int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 1

	for scanner.Scan() {
		if currentLine >= startLine && currentLine <= endLine {
			sb.WriteString(scanner.Text() + "\n")
		}
		if currentLine > endLine {
			break
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return sb.String(), nil
}
