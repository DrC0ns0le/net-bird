package bird

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	ASNumber int
}

func ParseBirdConfig(filename string) (Config, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Regular expression to match "local as" followed by numbers
	re := regexp.MustCompile(`local\s+as\s+(\d+)`)

	config := Config{}

	// Scan through each line
	for scanner.Scan() {
		line := scanner.Text()

		// Check if line contains the pattern we're looking for
		if strings.Contains(line, "local as") {
			// Extract the number using regex
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				// Parse the captured number
				var asNumber int
				_, err := fmt.Sscanf(matches[1], "%d", &asNumber)
				if err != nil {
					return config, fmt.Errorf("error parsing AS number: %v", err)
				}
				config.ASNumber = asNumber
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return config, fmt.Errorf("error reading file: %v", err)
	}

	return config, nil
}
