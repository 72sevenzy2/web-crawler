package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/72sevenzy2/web-crawler"
)

func main() {
	c := crawler.NewCrawler(10, true, 5)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(">")
		safe := scanner.Scan()
		if !safe {
			if err := scanner.Err(); err != nil {
				fmt.Println("err:", err.Error())
			}
			break
		}
		parts := strings.Fields(scanner.Text())

		switch strings.ToLower(parts[0]) {
		case "crawl":
			if len(parts) < 2 {
				fmt.Println("include a url to crawl through.")
				continue
			}

			c.Start(context.Background(), parts[1])
		}
	}
}
